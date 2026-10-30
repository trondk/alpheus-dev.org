/*
Copyright The Alpheus Authors.

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

package controller

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CASecret is the Secret, in the controller's namespace, with the CA that
// issues the signal hub's and the agents' certificates (GO-41). The first
// leader creates it; every replica reads it, so certificates stay valid
// after a failover.
const CASecret = "alpheus-signal-ca"

// Lifetimes of the CA and of the certificates it issues.
const (
	caLifetime   = 10 * 365 * 24 * time.Hour
	certLifetime = 90 * 24 * time.Hour
)

// CA issues certificates for the signal hub and the agents.
type CA struct {
	cert    *x509.Certificate
	key     *ecdsa.PrivateKey
	certPEM []byte
}

// LoadCA reads the CA from its Secret, creating it if it does not exist.
// The hub and the reconciler may both create it at startup; the one that
// loses waits until its client sees the winner's Secret.
func LoadCA(ctx context.Context, c client.Client, namespace string) (*CA, error) {
	for {
		var s corev1.Secret
		err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: CASecret}, &s)
		if err == nil {
			return parseCA(s.Data[corev1.TLSCertKey], s.Data[corev1.TLSPrivateKeyKey])
		}
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("signal CA: %w", err)
		}
		certPEM, keyPEM, err := newCA()
		if err != nil {
			return nil, err
		}
		s = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: CASecret},
			Type:       corev1.SecretTypeTLS,
			Data:       map[string][]byte{corev1.TLSCertKey: certPEM, corev1.TLSPrivateKeyKey: keyPEM},
		}
		err = c.Create(ctx, &s)
		if err == nil {
			return parseCA(certPEM, keyPEM)
		}
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("signal CA: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func newCA() (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "alpheus-signal-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(caLifetime),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	return encode(der, key)
}

func parseCA(certPEM, keyPEM []byte) (*CA, error) {
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("signal CA: %w", err)
	}
	key, ok := pair.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("signal CA: the key is not ECDSA")
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("signal CA: %w", err)
	}
	return &CA{cert: cert, key: key, certPEM: certPEM}, nil
}

// CertPEM is the CA certificate, for clients and servers to trust.
func (ca *CA) CertPEM() []byte { return ca.certPEM }

// Pool is a pool with only the CA.
func (ca *CA) Pool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	return pool
}

// Issue makes a certificate: a server certificate for dnsNames, or a client
// certificate when dnsNames is empty.
func (ca *CA) Issue(commonName string, dnsNames ...string) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	usage := x509.ExtKeyUsageClientAuth
	if len(dnsNames) > 0 {
		usage = x509.ExtKeyUsageServerAuth
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(certLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return nil, nil, err
	}
	return encode(der, key)
}

func encode(der []byte, key *ecdsa.PrivateKey) (certPEM, keyPEM []byte, err error) {
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), nil
}

func serial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	return n
}
