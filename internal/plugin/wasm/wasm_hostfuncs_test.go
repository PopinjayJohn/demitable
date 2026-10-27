package wasm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func writeTempVault(t *testing.T, files map[string]string) (*vault.Vault, string) {
	t.Helper()
	dir := t.TempDir()
	for p, c := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return v, dir
}

func callHost(t *testing.T, inst *Instance, fn func(m interface{}) uint32, req any) (Result, []byte) {
	// generic helper not used; keep for clarity
	return Result{}, nil
}

func TestHostVaultQuery(t *testing.T) {
	store := vault.NewStore(vault.Note{UID: "n1", Type: "note", Name: "T", Path: "a.md", Campaign: "c", Visibility: "party"})
	eng, _ := NewEngine(context.Background())
	defer eng.Close()
	inst, err := eng.Instantiate(context.Background(), nil, GoldenABIModule(), vault.GM(), store, []Grant{{ID: "vault.query"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(context.Background())
	req := []byte(`{}`)
	mem := inst.mod.Memory()
	mem.Write(1024, req)
	ptr := inst.hostVaultQuery(context.Background(), inst.mod, 1024, uint32(len(req)))
	out := readUntilNull(mem, ptr)
	var r Result
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("bad result: %v raw=%s", err, string(out))
	}
	if !r.OK {
		t.Fatalf("query failed: %+v", r.Error)
	}
}

func TestHostVaultAssetsScopeRefused(t *testing.T) {
	v, _ := writeTempVault(t, map[string]string{"a.txt": "hello"})
	eng, _ := NewEngine(context.Background())
	defer eng.Close()
	inst, err := eng.Instantiate(context.Background(), nil, GoldenABIModule(), vault.GM(), v, []Grant{{ID: "vault.assets", Scope: Scope{Paths: []string{"b.txt"}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(context.Background())
	req := []byte(`{"path":"a.txt"}`)
	mem := inst.mod.Memory()
	mem.Write(1024, req)
	ptr := inst.hostVaultAssets(context.Background(), inst.mod, 1024, uint32(len(req)))
	out := readUntilNull(mem, ptr)
	var r Result
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("bad result: %v raw=%s", err, string(out))
	}
	if r.OK {
		t.Fatal("expected scope refusal")
	}
	if r.Error == nil || r.Error.Code != "forbidden" {
		t.Fatalf("got %+v, want forbidden", r.Error)
	}
}

func TestHostVaultAssetsReads(t *testing.T) {
	v, _ := writeTempVault(t, map[string]string{"a.txt": "hello"})
	eng, _ := NewEngine(context.Background())
	defer eng.Close()
	inst, err := eng.Instantiate(context.Background(), nil, GoldenABIModule(), vault.GM(), v, []Grant{{ID: "vault.assets"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(context.Background())
	req := []byte(`{"path":"a.txt"}`)
	mem := inst.mod.Memory()
	mem.Write(1024, req)
	ptr := inst.hostVaultAssets(context.Background(), inst.mod, 1024, uint32(len(req)))
	out := readUntilNull(mem, ptr)
	var r Result
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("bad result: %v raw=%s", err, string(out))
	}
	if !r.OK {
		t.Fatalf("assets read failed: %+v", r.Error)
	}
	var body struct {
		Path string `json:"path"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal(r.Data, &body); err != nil {
		t.Fatal(err)
	}
	dec, _ := base64.StdEncoding.DecodeString(body.Data)
	if string(dec) != "hello" {
		t.Fatalf("got %q", string(dec))
	}
}

func TestHostLog(t *testing.T) {
	store := vault.NewStore()
	eng, _ := NewEngine(context.Background())
	defer eng.Close()
	inst, err := eng.Instantiate(context.Background(), nil, GoldenABIModule(), vault.GM(), store, []Grant{{ID: "log"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(context.Background())
	req := []byte(`{"level":"info","message":"hi"}`)
	mem := inst.mod.Memory()
	mem.Write(1024, req)
	ptr := inst.hostLog(context.Background(), inst.mod, 1024, uint32(len(req)))
	out := readUntilNull(mem, ptr)
	var r Result
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("bad result: %v raw=%s", err, string(out))
	}
	if !r.OK {
		t.Fatalf("log failed: %+v", r.Error)
	}
}

func TestHostNow(t *testing.T) {
	eng, err := NewEngine(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	fns := eng.hostMod.ExportedFunctionDefinitions()
	if _, ok := fns[FnNow]; !ok {
		t.Fatal("host_now not exported")
	}
}

func TestHostRandom(t *testing.T) {
	store := vault.NewStore()
	eng, _ := NewEngine(context.Background())
	defer eng.Close()
	inst, err := eng.Instantiate(context.Background(), nil, GoldenABIModule(), vault.GM(), store, []Grant{{ID: "log"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(context.Background())
	mem := inst.mod.Memory()
	mem.Write(1024, make([]byte, 16))
	ret := inst.hostRandom(context.Background(), inst.mod, 1024, 16)
	if ret != 0 {
		// hostRandom returns 0 on success; non-zero would be a writeResult pointer on failure
		out := readUntilNull(mem, ret)
		t.Fatalf("host_random failed: %s", string(out))
	}
	buf, _ := mem.Read(1024, 16)
	allZero := true
	for _, b := range buf {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("host_random wrote all zeros (unlikely)")
	}
}

func TestHostHTTPScopeRespected(t *testing.T) {
	store := vault.NewStore()
	eng, _ := NewEngine(context.Background())
	defer eng.Close()
	inst, err := eng.Instantiate(context.Background(), nil, GoldenABIModule(), vault.GM(), store, []Grant{{ID: "http.client", Scope: Scope{Paths: []string{"https://allowed.example/*"}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(context.Background())
	req := []byte(`{"url":"https://evil.example/x"}`)
	mem := inst.mod.Memory()
	mem.Write(1024, req)
	ptr := inst.hostHTTPFetch(context.Background(), inst.mod, 1024, uint32(len(req)))
	out := readUntilNull(mem, ptr)
	var r Result
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("bad result: %v raw=%s", err, string(out))
	}
	if r.OK {
		t.Fatal("expected scope refusal")
	}
	if r.Error == nil || r.Error.Code != "forbidden" {
		t.Fatalf("got %+v, want forbidden", r.Error)
	}
}

func TestHostHTTPSSRFRefused(t *testing.T) {
	store := vault.NewStore()
	eng, _ := NewEngine(context.Background())
	defer eng.Close()
	inst, err := eng.Instantiate(context.Background(), nil, GoldenABIModule(), vault.GM(), store, []Grant{{ID: "http.client"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close(context.Background())
	req := []byte(`{"url":"http://127.0.0.1/"}`)
	mem := inst.mod.Memory()
	mem.Write(1024, req)
	ptr := inst.hostHTTPFetch(context.Background(), inst.mod, 1024, uint32(len(req)))
	out := readUntilNull(mem, ptr)
	var r Result
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("bad result: %v raw=%s", err, string(out))
	}
	if r.OK {
		t.Fatal("expected ssrf refusal")
	}
}
