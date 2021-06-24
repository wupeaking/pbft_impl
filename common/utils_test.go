package common

import "testing"

func TestMerkels(t *testing.T) {
	v := [][]byte{{1}}
	t.Logf("%v", Merkel(v))
}
