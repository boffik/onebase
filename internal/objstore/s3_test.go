package objstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ptrBool(b bool) *bool { return &b }

// TestSignV4AWSVector checks our signature against AWS's documented
// "GET Object" Signature Version 4 example, so a regression in the signer is
// caught deterministically. See the AWS docs' worked example for these values.
func TestSignV4AWSVector(t *testing.T) {
	c := &Client{cfg: Config{
		Region:    "us-east-1",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}}

	req, err := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.URL.Opaque = "/test.txt"
	req.Header.Set("Range", "bytes=0-9")

	// Fixed time and empty-payload hash, exactly as in the AWS example.
	c.sign(req, emptyPayloadHash, time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))

	got := req.Header.Get("Authorization")
	const wantSig = "Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if !strings.Contains(got, wantSig) {
		t.Fatalf("signature mismatch:\n got  %q\n want to contain %q", got, wantSig)
	}
	if !strings.Contains(got, "SignedHeaders=host;range;x-amz-content-sha256;x-amz-date") {
		t.Errorf("unexpected SignedHeaders in %q", got)
	}
	if !strings.Contains(got, "Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request") {
		t.Errorf("unexpected Credential in %q", got)
	}
}

// testClient points a Client at an httptest server (path-style, plain http).
func testClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := New(Config{
		Endpoint:  strings.TrimPrefix(srv.URL, "http://"),
		Bucket:    "mybucket",
		AccessKey: "AKIA_TEST",
		SecretKey: "secret",
		UseSSL:    ptrBool(false),
		PathStyle: ptrBool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPutObjectRoundTrip(t *testing.T) {
	payload := []byte("backup-bytes-\x00\x01\x02")
	var gotBody []byte
	var gotAuth, gotSha, gotCT, gotPath, gotMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotSha = r.Header.Get("X-Amz-Content-Sha256")
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	err := c.PutObject(context.Background(), "prod/backup_x.db", bytes.NewReader(payload), int64(len(payload)), "application/octet-stream")
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/mybucket/prod/backup_x.db" {
		t.Errorf("path = %s, want /mybucket/prod/backup_x.db", gotPath)
	}
	if !bytes.Equal(gotBody, payload) {
		t.Errorf("body mismatch: got %q", gotBody)
	}
	if gotSha != unsignedPayload {
		t.Errorf("x-amz-content-sha256 = %q, want %q", gotSha, unsignedPayload)
	}
	if gotCT != "application/octet-stream" {
		t.Errorf("content-type = %q", gotCT)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=AKIA_TEST/") {
		t.Errorf("authorization = %q", gotAuth)
	}
}

func TestPutObjectServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>nope</Message></Error>`)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	err := c.PutObject(context.Background(), "k", strings.NewReader("x"), 1, "")
	if err == nil {
		t.Fatal("expected error on 403")
	}
	if !strings.Contains(err.Error(), "AccessDenied") || !strings.Contains(err.Error(), "403") {
		t.Errorf("error should surface S3 code/status: %v", err)
	}
}

func TestListKeysPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("list-type") != "2" {
			t.Errorf("missing list-type=2: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("prefix") != "prod/" {
			t.Errorf("prefix = %q", r.URL.Query().Get("prefix"))
		}
		w.Header().Set("Content-Type", "application/xml")
		if page == 0 && r.URL.Query().Get("continuation-token") == "" {
			page++
			fmt.Fprint(w, `<ListBucketResult><IsTruncated>true</IsTruncated>`+
				`<NextContinuationToken>TOK</NextContinuationToken>`+
				`<Contents><Key>prod/a</Key></Contents></ListBucketResult>`)
			return
		}
		if r.URL.Query().Get("continuation-token") != "TOK" {
			t.Errorf("expected continuation-token TOK, got %q", r.URL.Query().Get("continuation-token"))
		}
		fmt.Fprint(w, `<ListBucketResult><IsTruncated>false</IsTruncated>`+
			`<Contents><Key>prod/b</Key></Contents></ListBucketResult>`)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	keys, err := c.ListKeys(context.Background(), "prod/")
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 2 || keys[0] != "prod/a" || keys[1] != "prod/b" {
		t.Fatalf("keys = %v, want [prod/a prod/b]", keys)
	}
}

func TestDeleteObject(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	if err := c.DeleteObject(context.Background(), "prod/old.db"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if method != http.MethodDelete || path != "/mybucket/prod/old.db" {
		t.Errorf("got %s %s", method, path)
	}
}

func TestNewValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"no endpoint", Config{Bucket: "b", AccessKey: "a", SecretKey: "s"}},
		{"no bucket", Config{Endpoint: "e", AccessKey: "a", SecretKey: "s"}},
		{"no keys", Config{Endpoint: "e", Bucket: "b"}},
	} {
		if _, err := New(tc.cfg); err == nil {
			t.Errorf("%s: expected error", tc.name)
		}
	}
	c, err := New(Config{Endpoint: "e", Bucket: "b", AccessKey: "a", SecretKey: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if c.cfg.Region != "us-east-1" {
		t.Errorf("region default = %q, want us-east-1", c.cfg.Region)
	}
}
