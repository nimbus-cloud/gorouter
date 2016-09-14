package header_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"code.cloudfoundry.org/gorouter/common/secure/fakes"
	"code.cloudfoundry.org/gorouter/route_service/header"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Route Service Header", func() {
	var (
		crypto    = new(fakes.FakeCrypto)
		signature *header.Signature
	)

	BeforeEach(func() {
		crypto.DecryptStub = func(cipherText, nonce []byte) ([]byte, error) {
			decryptedStr := string(cipherText)

			decryptedStr = strings.Replace(decryptedStr, "encrypted", "", -1)
			decryptedStr = strings.Replace(decryptedStr, string(nonce), "", -1)
			return []byte(decryptedStr), nil
		}

		crypto.EncryptStub = func(plainText []byte) ([]byte, []byte, error) {
			nonce := []byte("some-nonce")
			cipherText := append(plainText, "encrypted"...)
			cipherText = append(cipherText, nonce...)
			return cipherText, nonce, nil
		}

		signature = &header.Signature{RequestedTime: time.Now()}
	})

	Describe("Build Signature and Metadata", func() {
		It("builds signature and metadata headers", func() {
			signatureHeader, metadata, err := header.BuildSignatureAndMetadata(crypto, signature)
			Expect(err).ToNot(HaveOccurred())
			Expect(signatureHeader).ToNot(BeNil())
			metadataDecoded, err := base64.URLEncoding.DecodeString(metadata)
			Expect(err).ToNot(HaveOccurred())
			metadataStruct := header.Metadata{}
			err = json.Unmarshal([]byte(metadataDecoded), &metadataStruct)
			Expect(err).ToNot(HaveOccurred())
			Expect(metadataStruct.Nonce).To(Equal([]byte("some-nonce")))
		})

		Context("when unable to encrypt the signature", func() {
			BeforeEach(func() {
				crypto.EncryptReturns([]byte{}, []byte{}, errors.New("No entropy"))
			})

			It("returns an error", func() {
				_, _, err := header.BuildSignatureAndMetadata(crypto, signature)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Parse signature from headers", func() {
		var (
			signatureHeader string
			metadataHeader  string
		)

		BeforeEach(func() {
			var err error
			signatureHeader, metadataHeader, err = header.BuildSignatureAndMetadata(crypto, signature)
			Expect(err).ToNot(HaveOccurred())
		})

		It("parses signature from signature and metadata headers", func() {
			decryptedSignature, err := header.SignatureFromHeaders(signatureHeader, metadataHeader, crypto)
			Expect(err).ToNot(HaveOccurred())
			Expect(signature.RequestedTime.Sub(decryptedSignature.RequestedTime)).To(Equal(time.Duration(0)))
		})
	})

})
