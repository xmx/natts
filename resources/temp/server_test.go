package temp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

func TestServer(t *testing.T) {
	cfg := generateTLSConfig()
	lis, err := quic.ListenAddr(":8989", cfg, nil)
	if err != nil {
		panic(err)
	}

	conn, err := lis.Accept(context.Background())
	if err != nil {
		panic(err)
	}
	defer conn.CloseWithError(0, "")

	stm, err := conn.AcceptStream(context.Background())
	if err != nil {
		panic(err)
	}

	_, err = stm.Write([]byte("A"))
	time.Sleep(time.Second)
	t.Log(err)
}

func TestClient(t *testing.T) {
	cfg := &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"echo"}}
	conn, err := quic.DialAddr(context.Background(), ":8989", cfg, nil)
	if err != nil {
		panic(err)
	}
	defer conn.CloseWithError(0, "")

	stm, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		panic(err)
	}
	defer stm.Close()

	buf := make([]byte, 1)
	stm.Read(buf)

	t.Log(buf[0])
}

func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"echo"},
	}
}
