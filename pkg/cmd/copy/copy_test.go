package copy

import (
	"testing"
)

func TestParseCopyArguments_Upload(t *testing.T) {
	ws, remotePath, localPath, isUpload, err := parseCopyArguments("./local.txt", "my-node:/tmp/dest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != "my-node" {
		t.Errorf("expected workspace my-node, got %s", ws)
	}
	if remotePath != "/tmp/dest" {
		t.Errorf("expected remotePath /tmp/dest, got %s", remotePath)
	}
	if localPath != "./local.txt" {
		t.Errorf("expected localPath ./local.txt, got %s", localPath)
	}
	if !isUpload {
		t.Error("expected isUpload=true")
	}
}

func TestParseCopyArguments_Download(t *testing.T) {
	ws, remotePath, localPath, isUpload, err := parseCopyArguments("my-node:/tmp/file", "./local.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != "my-node" {
		t.Errorf("expected workspace my-node, got %s", ws)
	}
	if remotePath != "/tmp/file" {
		t.Errorf("expected remotePath /tmp/file, got %s", remotePath)
	}
	if localPath != "./local.txt" {
		t.Errorf("expected localPath ./local.txt, got %s", localPath)
	}
	if isUpload {
		t.Error("expected isUpload=false")
	}
}

func TestParseCopyArguments_BothLocal(t *testing.T) {
	_, _, _, _, err := parseCopyArguments("./a", "./b")
	if err == nil {
		t.Fatal("expected error when both paths are local")
	}
}

func TestParseCopyArguments_BothRemote(t *testing.T) {
	_, _, _, _, err := parseCopyArguments("ws1:/a", "ws2:/b")
	if err == nil {
		t.Fatal("expected error when both paths are remote")
	}
}

func TestParseWorkspacePath_Local(t *testing.T) {
	ws, fp, err := parseWorkspacePath("/tmp/local/file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != "" {
		t.Errorf("expected empty workspace, got %s", ws)
	}
	if fp != "/tmp/local/file" {
		t.Errorf("expected /tmp/local/file, got %s", fp)
	}
}

func TestParseWorkspacePath_Remote(t *testing.T) {
	ws, fp, err := parseWorkspacePath("my-instance:/remote/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws != "my-instance" {
		t.Errorf("expected my-instance, got %s", ws)
	}
	if fp != "/remote/path" {
		t.Errorf("expected /remote/path, got %s", fp)
	}
}

func TestParseWorkspacePath_InvalidMultipleColons(t *testing.T) {
	_, _, err := parseWorkspacePath("ws:path:extra")
	if err == nil {
		t.Fatal("expected error for multiple colons")
	}
}
