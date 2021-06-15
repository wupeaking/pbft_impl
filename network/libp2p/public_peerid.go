package libp2p

import (
	"crypto/ecdsa"
	"crypto/x509"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	cryptogo "github.com/wupeaking/pbft_impl/crypto"
)

//自定义格式的公钥到libp2p的peerid转换

func PublicString2PeerID(key string) (string, error) {
	pub, err := cryptogo.LoadPublicKey(key)
	if err != nil {
		return "", err
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	p2pPubKey, err := crypto.UnmarshalECDSAPublicKey(pubBytes)
	if err != nil {
		return "", err
	}
	peerID, err := peer.IDFromPublicKey(p2pPubKey)
	return peerID.String(), err
}

func PublicKey2PeerID(pub *ecdsa.PublicKey) (string, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	p2pPubKey, err := crypto.UnmarshalECDSAPublicKey(pubBytes)
	if err != nil {
		return "", err
	}
	peerID, err := peer.IDFromPublicKey(p2pPubKey)
	return peerID.String(), err
}

func PrivateKey2PeerID(pri *ecdsa.PrivateKey) (string, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(&pri.PublicKey)
	if err != nil {
		return "", err
	}
	p2pPubKey, err := crypto.UnmarshalECDSAPublicKey(pubBytes)
	if err != nil {
		return "", err
	}
	peerID, err := peer.IDFromPublicKey(p2pPubKey)
	return peerID.String(), err
}
