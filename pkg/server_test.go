package pkg

import "testing"

func TestBuildPath(t *testing.T) {
	if "/a" != buildPath("a", "") {
		t.Fatal()
	}
	if "/a" != buildPath("/a/", "") {
		t.Fatal()
	}

	if "/a/b" != buildPath("/a/", "/b/") {
		t.Fatal()
	}
}
