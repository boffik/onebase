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

// writeAll пишет тело фикстур-сервера; ошибку записи помечает как сбой теста.
func writeAll(t *testing.T, w io.Writer, b []byte) {
	t.Helper()
	if _, err := w.Write(b); err != nil {
		t.Errorf("write fixture response: %v", err)
	}
}

// closeChecked закрывает ресурс; ошибку закрытия помечает как сбой теста.
func closeChecked(t *testing.T, c io.Closer) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Errorf("close: %v", err)
	}
}

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
		writeAll(t, w, []byte(`<Error><Code>AccessDenied</Code><Message>nope</Message></Error>`))
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
			writeAll(t, w, []byte(`<ListBucketResult><IsTruncated>true</IsTruncated>`+
				`<NextContinuationToken>TOK</NextContinuationToken>`+
				`<Contents><Key>prod/a</Key></Contents></ListBucketResult>`))
			return
		}
		if r.URL.Query().Get("continuation-token") != "TOK" {
			t.Errorf("expected continuation-token TOK, got %q", r.URL.Query().Get("continuation-token"))
		}
		writeAll(t, w, []byte(`<ListBucketResult><IsTruncated>false</IsTruncated>`+
			`<Contents><Key>prod/b</Key></Contents></ListBucketResult>`))
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

func TestGetObjectRoundTrip(t *testing.T) {
	payload := []byte("image-bytes-\xff\x00\x10")
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		w.WriteHeader(http.StatusOK)
		writeAll(t, w, payload)
	}))
	defer srv.Close()

	c := testClient(t, srv)
	rc, size, err := c.GetObject(context.Background(), "blobs/abc")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer closeChecked(t, rc)
	if gotPath != "/mybucket/blobs/abc" {
		t.Errorf("path = %s", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=AKIA_TEST/") {
		t.Errorf("authorization = %q", gotAuth)
	}
	if size != int64(len(payload)) {
		t.Errorf("size = %d, want %d", size, len(payload))
	}
	body, _ := io.ReadAll(rc)
	if !bytes.Equal(body, payload) {
		t.Errorf("body mismatch: %q", body)
	}
}

func makePayload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

func TestOpenReadSeeker_FullAndRange(t *testing.T) {
	payload := makePayload(1000)
	// Upstream S3 stand-in that honors Range (http.ServeContent does the parsing).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "x", time.Time{}, bytes.NewReader(payload))
	}))
	defer srv.Close()
	c := testClient(t, srv)

	// Seek to end returns the known size (no network needed).
	rs := c.OpenReadSeeker(context.Background(), "blobs/x", int64(len(payload)))
	if sz, err := rs.Seek(0, io.SeekEnd); err != nil || sz != int64(len(payload)) {
		t.Fatalf("Seek end = %d, %v; want %d", sz, err, len(payload))
	}
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	full, _ := io.ReadAll(rs)
	closeChecked(t, rs)
	if !bytes.Equal(full, payload) {
		t.Fatalf("full read mismatch (%d bytes)", len(full))
	}

	// Seek then read a middle window.
	rs2 := c.OpenReadSeeker(context.Background(), "blobs/x", int64(len(payload)))
	defer closeChecked(t, rs2)
	if _, err := rs2.Seek(400, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	win := make([]byte, 100)
	if _, err := io.ReadFull(rs2, win); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if !bytes.Equal(win, payload[400:500]) {
		t.Fatalf("range window mismatch")
	}
}

// TestOpenReadSeeker_ServeContent proves the real flow: an attachment download
// handler using http.ServeContent over the streaming seeker serves both a full
// download and a client Range request correctly.
func TestOpenReadSeeker_ServeContent(t *testing.T) {
	payload := makePayload(2000)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "x", time.Time{}, bytes.NewReader(payload))
	}))
	defer upstream.Close()
	c := testClient(t, upstream)

	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs := c.OpenReadSeeker(r.Context(), "blobs/x", int64(len(payload)))
		defer closeChecked(t, rs)
		http.ServeContent(w, r, "file.bin", time.Time{}, rs)
	}))
	defer dl.Close()

	// Full download.
	resp, err := http.Get(dl.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close() // errcheck+bodyclose: явное закрытие тела ответа
	if resp.StatusCode != 200 || !bytes.Equal(body, payload) {
		t.Fatalf("full: status=%d len=%d", resp.StatusCode, len(body))
	}

	// Range request bytes=500-799.
	req, _ := http.NewRequest("GET", dl.URL, nil)
	req.Header.Set("Range", "bytes=500-799")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	part, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close() // errcheck+bodyclose: явное закрытие тела ответа
	if resp2.StatusCode != http.StatusPartialContent {
		t.Fatalf("range status = %d, want 206", resp2.StatusCode)
	}
	if !bytes.Equal(part, payload[500:800]) {
		t.Fatalf("range body mismatch (%d bytes)", len(part))
	}
}

func TestGetObjectNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		writeAll(t, w, []byte(`<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`))
	}))
	defer srv.Close()

	c := testClient(t, srv)
	_, _, err := c.GetObject(context.Background(), "blobs/missing")
	if err == nil || !strings.Contains(err.Error(), "NoSuchKey") {
		t.Fatalf("expected NoSuchKey error, got %v", err)
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
