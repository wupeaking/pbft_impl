package libp2p

import (
	"testing"

	cryptogo "github.com/wupeaking/pbft_impl/crypto"
)

//自定义格式的公钥到libp2p的peerid转换

func TestPublicKey2PeerID(t *testing.T) {
	t.Logf("test-----------------")
	pri, err := cryptogo.LoadPrivateKey("0xa665c8da936eba27a48eae8c5f6d862e017c8b47715a11b0267570631af09d59")
	if err != nil {
		t.Fatal(err)
	}

	id, err := PublicKey2PeerID(&pri.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if id != "QmTk5Qp3YCdxinjwzFyEsivKv2AYFzvhyXAspMTszZ42xF" {
		t.Fatal("id: ", id)
	}

	t.Log(PublicString2PeerID("0xc4024ffd0b42495f49002b5da606512aee341c53e43a641b7d8efac8e29f6ed2d5c6449fe4343f41c5216a84ea9dd43e07daeeadb38556bb19527ce699394cd7"))
	t.Log(PublicString2PeerID("0x302404eeb2e3d1e75f78f426836cb6ee741d735153e441f1f43fbec55b4482c6d2d59017e608b995ba32255b31c49b646d59834537b9c2efb7cd66c64250c5b2"))
	t.Log(PublicString2PeerID("0x5ca153355f800c66150130b8becb951856e408555829eb07de89d3ed35fdd85872923fd9c51444ace5df3d6ce331da676a5e90596e7952f3f4a05c623bc00d77"))

}
