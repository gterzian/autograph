package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ThalesIgnite/crypto11"
)

func usage() {
	fmt.Printf(`make an end-entity certificate on the hsm for use in content signature

usage: go run make-hsm-ee.go -i <intermediate_label> -a <appname> (-p <hsm_lib_path> -t <hsm_type> -s <hsm_pin>)

eg. $ go run make-hsm-ee.go -i csinter1555704936 -a normandy
`)

	log.Fatal()
}
func main() {
	var (
		interKeyName, appName, hsmPath, hsmType, hsmPin string
		slots                                           []uint
		err                                             error
	)
	flag.StringVar(&interKeyName, "i", "",
		"label of the private key of the intermediate in the hsm")
	flag.StringVar(&appName, "a", "",
		"name of the application the end-entity is for (eg. remote-settings)")
	flag.StringVar(&hsmPath, "p", "/usr/lib/softhsm/libsofthsm2.so",
		"path to the hsm pkcs11 library")
	flag.StringVar(&hsmType, "t", "test",
		"type of the hsm (use 'cavium' for cloudhsm)")
	flag.StringVar(&hsmPin, "s", "0000",
		"pin to log into the hsm (use 'user:pass' on cloudhsm)")
	flag.Parse()

	if appName == "" || interKeyName == "" {
		usage()
	}

	p11Ctx, err := crypto11.Configure(&crypto11.PKCS11Config{
		Path:       hsmPath,
		TokenLabel: hsmType,
		Pin:        hsmPin,
	})
	if err != nil {
		log.Fatal(err)
	}
	slots, err = p11Ctx.GetSlotList(true)
	if err != nil {
		log.Fatalf("Failed to list PKCS#11 Slots: %s", err.Error())
	}
	log.Printf("Using HSM on slot %d", slots[0])
	interPriv, err := crypto11.FindKeyPair(nil, []byte(interKeyName))
	if err != nil {
		log.Fatal(err)
	}

	// make a keypair for the end-entity
	eePriv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	eePub := eePriv.Public()

	certTpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			Organization:       []string{"Mozilla Corporation"},
			OrganizationalUnit: []string{"Cloud Services"},
			Country:            []string{"US"},
			Province:           []string{"California"},
			Locality:           []string{"Mountain View"},
			CommonName:         appName + ".content-signature.mozilla.org",
		},
		DNSNames:           []string{appName + ".content-signature.mozilla.org"},
		NotBefore:          time.Now().AddDate(0, 0, -30), // start 30 days ago
		NotAfter:           time.Now().AddDate(0, 0, 60),  // valid for 60 days
		SignatureAlgorithm: x509.ECDSAWithSHA384,
		IsCA:               false,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		KeyUsage:           x509.KeyUsageDigitalSignature,
	}
	eeCertBytes, err := x509.CreateCertificate(
		rand.Reader, certTpl, certTpl, eePub, interPriv)
	if err != nil {
		log.Fatalf("create cert failed: %v", err)
	}

	var eePem bytes.Buffer
	err = pem.Encode(&eePem, &pem.Block{Type: "CERTIFICATE", Bytes: eeCertBytes})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", eePem.Bytes())

	eePrivBytes, err := x509.MarshalECPrivateKey(eePriv)
	if err != nil {
		log.Fatal(err)
	}
	var eePrivPem bytes.Buffer
	err = pem.Encode(&eePrivPem,
		&pem.Block{Type: "EC PRIVATE KEY", Bytes: eePrivBytes})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", eePrivPem.Bytes())
}
