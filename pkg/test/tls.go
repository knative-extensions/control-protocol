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

package test

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"knative.dev/control-protocol/pkg/certificates"
	"knative.dev/control-protocol/pkg/network"
)

func MustGenerateTestTLSConf(t *testing.T, ctx context.Context) (func() (*tls.Config, error), *tls.Dialer) {
	caKP, err := certificates.CreateCACerts(ctx, 24*time.Hour)
	require.NoError(t, err)
	caCert, caPrivateKey, err := caKP.Parse()
	require.NoError(t, err)

	serverTLSConf := mustGenerateTLSServerConf(t, ctx, caPrivateKey, caCert, "myns")
	clientTLSConf := mustGenerateTLSClientConf(t, ctx, caPrivateKey, caCert, "myns")

	tlsDialer := &tls.Dialer{
		NetDialer: &net.Dialer{
			KeepAlive: network.KeepAlive,
			Deadline:  time.Time{},
		},
		Config: clientTLSConf,
	}
	return serverTLSConf, tlsDialer
}

func mustGenerateTLSServerConf(t *testing.T, ctx context.Context, caKey *rsa.PrivateKey, caCertificate *x509.Certificate, namespace string) func() (*tls.Config, error) {
	return func() (*tls.Config, error) {
		dataPlaneKeyPair, err := certificates.CreateCert(ctx, caKey, caCertificate, 24*time.Hour, certificates.DataPlaneNamePrefix+namespace, certificates.LegcayFakeDnsName)
		require.NoError(t, err)

		dataPlaneCert, err := tls.X509KeyPair(dataPlaneKeyPair.CertBytes(), dataPlaneKeyPair.PrivateKeyBytes())
		require.NoError(t, err)

		certPool := x509.NewCertPool()
		certPool.AddCert(caCertificate)
		return &tls.Config{
			Certificates: []tls.Certificate{dataPlaneCert},
			ClientCAs:    certPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}, nil
	}
}

func mustGenerateTLSClientConf(t *testing.T, ctx context.Context, caKey *rsa.PrivateKey, caCertificate *x509.Certificate, namespace string) *tls.Config {
	controlPlaneKeyPair, err := certificates.CreateCert(ctx, caKey, caCertificate, 24*time.Hour, certificates.DataPlaneNamePrefix+namespace, certificates.LegcayFakeDnsName)
	require.NoError(t, err)

	controlPlaneCert, err := tls.X509KeyPair(controlPlaneKeyPair.CertBytes(), controlPlaneKeyPair.PrivateKeyBytes())
	require.NoError(t, err)

	certPool := x509.NewCertPool()
	certPool.AddCert(caCertificate)
	return &tls.Config{
		Certificates: []tls.Certificate{controlPlaneCert},
		RootCAs:      certPool,
		ServerName:   certificates.DataPlaneNamePrefix + namespace,
	}
}
