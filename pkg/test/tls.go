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
		dataPlaneEdgeKeyPair, err := certificates.CreateCert(ctx, caKey, caCertificate, 24*time.Hour, certificates.DataPlaneEdgeName(namespace), certificates.LegacyFakeDnsName)
		require.NoError(t, err)

		dataPlaneEdgeCert, err := tls.X509KeyPair(dataPlaneEdgeKeyPair.CertBytes(), dataPlaneEdgeKeyPair.PrivateKeyBytes())
		require.NoError(t, err)

		certPool := x509.NewCertPool()
		certPool.AddCert(caCertificate)
		return &tls.Config{
			Certificates: []tls.Certificate{dataPlaneEdgeCert},
			ClientCAs:    certPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
			VerifyConnection: func(cs tls.ConnectionState) error {
				err := cs.PeerCertificates[0].VerifyHostname(certificates.DataPlaneRoutingName(""))
				return err
			},
		}, nil
	}
}

func mustGenerateTLSClientConf(t *testing.T, ctx context.Context, caKey *rsa.PrivateKey, caCertificate *x509.Certificate, namespace string) *tls.Config {
	dataPlaneRoutingKeyPair, err := certificates.CreateCert(ctx, caKey, caCertificate, 24*time.Hour, certificates.DataPlaneRoutingName(""))
	require.NoError(t, err)

	dataPlaneRoutingCert, err := tls.X509KeyPair(dataPlaneRoutingKeyPair.CertBytes(), dataPlaneRoutingKeyPair.PrivateKeyBytes())
	require.NoError(t, err)

	certPool := x509.NewCertPool()
	certPool.AddCert(caCertificate)
	return &tls.Config{
		Certificates: []tls.Certificate{dataPlaneRoutingCert},
		RootCAs:      certPool,
		ServerName:   certificates.DataPlaneEdgeName(namespace),
	}
}
