package gitverify

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/supply-chain-tools/go-sandbox/hashset"
	"golang.org/x/crypto/ssh"
)

type SSHSig struct {
	MagicPreamble [6]byte
	SigVersion    uint32
	PublicKey     string
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Signature     string
}

func validateSSH(content string, signature string, sshPublicKeys map[string]*ssh.PublicKey, config *RepoConfig) error {
	if !config.allowSSHSignatures {
		return fmt.Errorf("SSH signatures not allowed")
	}

	sshSig, err := decodeAndParseSSHSignature(signature)
	if err != nil {
		return err
	}

	trustedKey, found := sshPublicKeys[sshSig.PublicKey]
	if found {
		err = verifySignature(*trustedKey, content, sshSig, sshExpectedNamespace, config.allowSSHSHA256, config.requireSHA512)
		if err != nil {
			return err
		}

		if config.requireSSHUserPresent || config.requireSSHUserVerified {
			u2fSignature, err := parseU2FSignature(sshSig)
			if err != nil {
				format, formatErr := getSignatureFormat(sshSig)
				if formatErr != nil {
					return formatErr
				}

				if !(*format == ssh.KeyAlgoSKED25519 || *format == ssh.KeyAlgoECDSA256) {
					return fmt.Errorf("unsupported public key type %s for user present/verified", *format)
				}

				return err
			}

			if config.requireSSHUserPresent && !u2fSignature.userPresent() {
				return fmt.Errorf("user present missing")
			}

			if config.requireSSHUserVerified && !u2fSignature.userVerified() {
				return fmt.Errorf("user verified missing")
			}
		}
	} else {
		return fmt.Errorf("matching SSH key not found")
	}

	return nil
}

func getSignatureFormat(sshSig *SSHSig) (*string, error) {
	sig := &ssh.Signature{}
	err := ssh.Unmarshal([]byte(sshSig.Signature), sig)
	if err != nil {
		return nil, err
	}

	keyType, err := keyTypeFromSignatureFormat(sig.Format)
	if err != nil {
		return nil, err
	}

	return &keyType, err
}

func verifySSHSignature(key string, signature string, data string, expectedNamespace string, allowSHA256 bool, requireSHA512 bool, allowedSSHKeyFormats hashset.Set[string]) error {
	publicKey, _, err := decodeAndParseSSHPublicKey(key, allowedSSHKeyFormats)
	if err != nil {
		return err
	}

	sshSig, err := decodeAndParseSSHSignature(signature)
	if err != nil {
		return err
	}

	err = verifySignature(publicKey, data, sshSig, expectedNamespace, allowSHA256, requireSHA512)
	if err != nil {
		return err
	}

	return nil
}

func decodeAndParseSSHPublicKey(key string, allowedSSHKeyFormats hashset.Set[string]) (ssh.PublicKey, []byte, error) {
	parts := strings.Split(key, " ")
	if len(parts) < 2 {
		return nil, nil, fmt.Errorf("invalid SSH public key")
	}

	rawKey, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode key: %v", err)
	}

	publicKey, err := ssh.ParsePublicKey(rawKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	if publicKey.Type() != parts[0] {
		return nil, nil, fmt.Errorf("inconsistent format for SSH public key '%s'", key)
	}

	if !isSupportedKeyFormat(publicKey.Type(), allowedSSHKeyFormats) {
		return nil, nil, fmt.Errorf("unsupported key format '%s'", publicKey.Type())
	}

	return publicKey, rawKey, nil
}

func decodeAndParseSSHSignature(signature string) (*SSHSig, error) {
	rawSignature, err := unwrapSshSignature(signature)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap signature: %v", err)
	}

	rawSig, err := base64.StdEncoding.DecodeString(rawSignature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %v", err)
	}

	sshSig := &SSHSig{}
	err = ssh.Unmarshal(rawSig, sshSig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal signature: %w", err)
	}

	return sshSig, nil
}

func verifySignature(maintainerAllowedKey ssh.PublicKey, message string, signature *SSHSig, expectedNamespace string, allowSHA256 bool, requireSHA512 bool) error {
	if !bytes.Equal(signature.MagicPreamble[:], sshExpectedMagicPreamble) {
		return fmt.Errorf("incorrect SSH magic preamble")
	}

	if signature.SigVersion != 1 {
		return fmt.Errorf("unsupported SSH signature version %d", signature.SigVersion)
	}

	if signature.Namespace != expectedNamespace {
		return fmt.Errorf("unsupported SSH namespace, expected '%s', got '%s'", expectedNamespace, signature.Namespace)
	}

	if signature.Reserved != sshExpectedReservedField {
		return fmt.Errorf("non-empty reserved SSH field")
	}

	if allowSHA256 && requireSHA512 {
		return fmt.Errorf("invalid arguments: allowSHA256 and requireSHA512 cannot both be true")
	}

	if requireSHA512 && signature.HashAlgorithm != "sha512" {
		return fmt.Errorf("SHA-512 required for SSH, got: %s", signature.HashAlgorithm)
	}

	var h []byte
	switch signature.HashAlgorithm {
	case "sha256":
		if !allowSHA256 {
			return fmt.Errorf("hash algorithm SHA-256 not allowed for SSH")
		}
		r := sha256.Sum256([]byte(message))
		h = r[:]
	case "sha512":
		r := sha512.Sum512([]byte(message))
		h = r[:]
	default:
		return fmt.Errorf("unsupported hash algorithm: %s", signature.HashAlgorithm)
	}

	sshSig := SshSig{
		Namespace:     expectedNamespace,
		Reserved:      sshExpectedReservedField,
		HashAlgorithm: signature.HashAlgorithm,
		Hash:          string(h),
	}

	signedBlob := ssh.Marshal(sshSig)
	signedBlob = append(sshExpectedMagicPreamble, signedBlob...)

	sig := &ssh.Signature{}
	err := ssh.Unmarshal([]byte(signature.Signature), sig)
	if err != nil {
		return err
	}

	if shouldHaveEmptyRest(sig.Format) {
		if len(sig.Rest) != 0 {
			return fmt.Errorf("rest field not empty")
		}
	}

	expectedKeyType, err := keyTypeFromSignatureFormat(sig.Format)
	if err != nil {
		return err
	}

	if expectedKeyType != maintainerAllowedKey.Type() {
		return fmt.Errorf("signature format '%s' does not match key type '%s'", sig.Format, maintainerAllowedKey.Type())
	}

	err = maintainerAllowedKey.Verify(signedBlob, sig)
	if err != nil {
		return err
	}

	return nil
}

func keyTypeFromSignatureFormat(format string) (string, error) {
	switch format {
	case ssh.KeyAlgoSKED25519:
		// USES SHA-256 internally
		return format, nil
	case ssh.KeyAlgoSKECDSA256:
		// USES SHA-256 internally
		return format, nil
	case ssh.KeyAlgoED25519:
		// USES SHA-512 internally
		return format, nil
	case ssh.KeyAlgoECDSA256:
		// USES SHA-256 internally
		return format, nil
	case ssh.KeyAlgoECDSA384:
		// USES SHA-384 internally
		return format, nil
	case ssh.KeyAlgoECDSA521:
		// USES SHA-512 internally
		return format, nil
	case ssh.KeyAlgoRSASHA512:
		// USES SHA-512 internally
		return "ssh-rsa", nil
	case ssh.KeyAlgoRSASHA256:
		// USES SHA-256 internally
		return "ssh-rsa", nil
	default:
		return "", fmt.Errorf("unsupported key format '%s'", format)
	}
}

func isSupportedKeyFormat(format string, allowedSSHKeyFormats hashset.Set[string]) bool {
	return allowedSSHKeyFormats.Contains(format)
}

func defaultSSHKeyFormats() hashset.Set[string] {
	return hashset.New[string](
		ssh.KeyAlgoSKED25519,
		ssh.KeyAlgoSKECDSA256,
		ssh.KeyAlgoED25519,
		ssh.KeyAlgoECDSA256,
		ssh.KeyAlgoECDSA384,
		ssh.KeyAlgoECDSA521,
		ssh.KeyAlgoRSA)
}

func shouldHaveEmptyRest(format string) bool {
	switch format {
	case ssh.KeyAlgoSKED25519:
		return false
	case ssh.KeyAlgoSKECDSA256:
		return false
	default:
		return true
	}
}

func unwrapSshSignature(signature string) (string, error) {
	header := "-----BEGIN SSH SIGNATURE-----\n"
	footer := "-----END SSH SIGNATURE-----"

	signature = strings.Trim(signature, "\n ")

	if !strings.HasPrefix(signature, header) {
		return "", fmt.Errorf("signature does not start with header")
	}

	if !strings.HasSuffix(signature, footer) {
		return "", fmt.Errorf("signature does not end with footer")
	}

	subsetStart := len(header)
	subsetEnd := len(signature) - len(footer) - 1
	if subsetEnd < subsetStart {
		return "", fmt.Errorf("signature is too short")
	}

	subset := signature[subsetStart:subsetEnd]
	result := strings.ReplaceAll(subset, "\n", "")
	return result, nil
}

type U2FSignature struct {
	// https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.u2f
	Type      string
	Signature []byte
	Flags     byte
	Counter   uint32
}

func parseU2FSignature(sshSig *SSHSig) (*U2FSignature, error) {
	signature := &U2FSignature{}

	err := ssh.Unmarshal([]byte(sshSig.Signature), signature)
	if err != nil {
		return nil, err
	}

	return signature, nil
}

func (u *U2FSignature) userPresent() bool {
	return (u.Flags & 1) != 0
}

func (u *U2FSignature) userVerified() bool {
	return (u.Flags >> 2 & 1) != 0
}

type SshSig struct {
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Hash          string
}
