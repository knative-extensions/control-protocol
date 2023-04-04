/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sample

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	strings "strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"

	"knative.dev/control-protocol/pkg/certificates"
)

const (
	caExpirationInterval = time.Hour * 24 * 365 * 10 // 10 years
	expirationInterval   = time.Hour * 24 * 30       // 30 days
	rotationThreshold    = 10 * time.Minute

	// certificates used by control elements such as autoscaler, ingress controller
	controlPlaneSecretType = "control-plane"

	// certificates used by trusted data routing elements such as activator, ingress gw
	dataPlaneRoutingSecretType = "data-plane-routing"

	// certificates used by edges acting as senders and receivers in the data-plane such as queue
	dataPlaneEdgeSecretType = "data-plane-edge"

	// Deprecated used by any data plane element
	dataPlaneDeprecatedSecretType = "data-plane"
)

// Reconciler reconciles a SampleSource object
type reconciler struct {
	client              kubernetes.Interface
	secretLister        listerv1.SecretLister
	caSecretName        string
	secretTypeLabelName string
	enqueueAfter        func(key types.NamespacedName, delay time.Duration)

	logger *zap.SugaredLogger
}

// Check that our Reconciler implements Interface
var _ Interface = (*reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *reconciler) ReconcileKind(ctx context.Context, secret *corev1.Secret) pkgreconciler.Event {
	// This should not happen, but it happens :) https://github.com/knative/pkg/issues/1891
	if !r.shouldReconcile(secret) {
		r.logger.Infof("Skipping reconciling secret %q:%q", secret.Namespace, secret.Name)
		return nil
	}
	r.logger.Infof("Updating secret %q:%q", secret.Namespace, secret.Name)

	// Reconcile CA secret first
	caSecret, err := r.secretLister.Secrets(system.Namespace()).Get(r.caSecretName)
	if apierrors.IsNotFound(err) {
		// The secret should be created explicitly by a higher-level system
		// that's responsible for install/updates.  We simply populate the
		// secret information.
		return nil
	} else if err != nil {
		r.logger.Errorf("Error accessing CA certificate secret %q %q: %v", system.Namespace(), r.caSecretName, err)
		return err
	}
	caCert, caPk, err := parseAndValidateSecret(caSecret, false)
	if err != nil {
		r.logger.Infof("CA cert invalid: %v", err)

		// We need to generate a new CA cert, then shortcircuit the reconciler
		keyPair, err := certificates.CreateCACerts(ctx, caExpirationInterval)
		if err != nil {
			return fmt.Errorf("cannot generate the CA cert: %v", err)
		}
		return r.commitUpdatedSecret(ctx, caSecret, keyPair, nil)
	}

	// Reconcile the provided secret
	var sans []string
	switch secret.Labels[r.secretTypeLabelName] {
	case controlPlaneSecretType:
		sans = []string{certificates.ControlPlaneName}
	case dataPlaneRoutingSecretType:
		routingId := secret.Labels[secretRoutingId]
		san := certificates.DataPlaneRoutingName(routingId)
		sans = []string{san, certificates.LegacyFakeDnsName}
	case dataPlaneEdgeSecretType:
		sans = []string{certificates.DataPlaneEdgeName(secret.Namespace), certificates.LegacyFakeDnsName}
	case dataPlaneDeprecatedSecretType:
		sans = []string{certificates.LegacyFakeDnsName}
	default:
		return fmt.Errorf("unknown cert type: %v", r.secretTypeLabelName)
	}

	cert, _, err := parseAndValidateSecret(secret, true, sans...)
	if err != nil {
		r.logger.Infof("Secret invalid: %v", err)
		// Check the secret to reconcile type

		var keyPair *certificates.KeyPair
		keyPair, err = certificates.CreateCert(ctx, caPk, caCert, expirationInterval, sans...)
		if err != nil {
			return fmt.Errorf("cannot generate the cert: %v", err)
		}
		err = r.commitUpdatedSecret(ctx, secret, keyPair, caSecret.Data[certificates.SecretCertKey])
		if err != nil {
			return err
		}
		cert, _, err = certificates.ParseCert(keyPair.CertBytes(), keyPair.PrivateKeyBytes())
		if err != nil {
			return err
		}
	}

	r.enqueueBeforeExpiration(secret, cert)

	return nil
}

func parseAndValidateSecret(secret *corev1.Secret, shouldContainCaCert bool, sans ...string) (*x509.Certificate, *rsa.PrivateKey, error) {
	certBytes, ok := secret.Data[certificates.SecretCertKey]
	if !ok {
		return nil, nil, fmt.Errorf("missing cert bytes")
	}
	pkBytes, ok := secret.Data[certificates.SecretPKKey]
	if !ok {
		return nil, nil, fmt.Errorf("missing pk bytes")
	}
	if shouldContainCaCert {
		if _, ok := secret.Data[certificates.SecretCaCertKey]; !ok {
			return nil, nil, fmt.Errorf("missing ca cert bytes")
		}
	}

	cert, caPk, err := certificates.ParseCert(certBytes, pkBytes)
	if err != nil {
		return nil, nil, err
	}
	if err := certificates.ValidateCert(cert, rotationThreshold); err != nil {
		return nil, nil, err
	}

OUTER:
	for _, san := range sans {
		for _, certSan := range cert.DNSNames {
			if strings.EqualFold(certSan, san) {
				continue OUTER
			}
		}
		return nil, nil, fmt.Errorf("missing san %q", san)
	}

	return cert, caPk, nil
}

func (r *reconciler) enqueueBeforeExpiration(secret *corev1.Secret, cert *x509.Certificate) {
	when := cert.NotAfter.Add(-rotationThreshold).Add(1 * time.Second) // Make sure to enqueue it after the rotation threshold
	r.enqueueAfter(types.NamespacedName{
		Namespace: secret.Namespace,
		Name:      secret.Name,
	}, time.Until(when))
}

func (r *reconciler) commitUpdatedSecret(ctx context.Context, secret *corev1.Secret, keyPair *certificates.KeyPair, caCert []byte) error {
	// Don't modify the informer copy.
	secret = secret.DeepCopy()

	secret.Data = make(map[string][]byte, 3)
	secret.Data[certificates.CertName] = keyPair.CertBytes()
	secret.Data[certificates.PrivateKeyName] = keyPair.PrivateKeyBytes()
	secret.Data[certificates.SecretCertKey] = keyPair.CertBytes()
	secret.Data[certificates.SecretPKKey] = keyPair.PrivateKeyBytes()
	if caCert != nil {
		secret.Data[certificates.SecretCaCertKey] = caCert
		secret.Data[certificates.CaCertName] = caCert
	}

	_, err := r.client.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

func (r *reconciler) shouldReconcile(secret *corev1.Secret) bool {
	// Is CA secret?
	if secret.Name == r.caSecretName && secret.Namespace == system.Namespace() {
		return false
	}

	_, hasLabel := secret.Labels[r.secretTypeLabelName]
	return hasLabel
}
