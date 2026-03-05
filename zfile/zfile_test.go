package zfile

import (
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
)

// ============================================================
// Exists 单元测试
// ============================================================

func TestExists_FileExists(t *testing.T) {
	// 创建临时文件
	f, err := os.CreateTemp("", "test_exists_*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	if !Exists(f.Name()) {
		t.Error("existing file should return true")
	}
}

func TestExists_FileNotExists(t *testing.T) {
	if Exists("/nonexistent_path_abcdef_12345") {
		t.Error("non-existent file should return false")
	}
}

func TestExists_DirExists(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_exists_dir_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if !Exists(dir) {
		t.Error("existing directory should return true")
	}
}

// ============================================================
// Mkdir 单元测试
// ============================================================

func TestMkdir_CreateNew(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "test_mkdir_new_"+t.Name())
	defer os.RemoveAll(dir)

	err := Mkdir(dir)
	if err != nil {
		t.Fatalf("Mkdir error: %v", err)
	}
	if !Exists(dir) {
		t.Error("directory should exist after Mkdir")
	}
}

func TestMkdir_Nested(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "test_mkdir_nested_"+t.Name(), "a", "b", "c")
	defer os.RemoveAll(filepath.Join(os.TempDir(), "test_mkdir_nested_"+t.Name()))

	err := Mkdir(dir)
	if err != nil {
		t.Fatalf("Mkdir nested error: %v", err)
	}
	if !Exists(dir) {
		t.Error("nested directory should exist after Mkdir")
	}
}

func TestMkdir_AlreadyExists(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_mkdir_exists_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// 再次创建不应报错
	err = Mkdir(dir)
	if err != nil {
		t.Errorf("Mkdir on existing dir should not error: %v", err)
	}
}

// ============================================================
// CreateFile 单元测试
// ============================================================

func TestCreateFile_Basic(t *testing.T) {
	path := filepath.Join(os.TempDir(), "test_create_file_"+t.Name()+".txt")
	defer os.Remove(path)

	err := CreateFile(path, "hello world")
	if err != nil {
		t.Fatalf("CreateFile error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", data)
	}
}

func TestCreateFile_Overwrite(t *testing.T) {
	path := filepath.Join(os.TempDir(), "test_create_overwrite_"+t.Name()+".txt")
	defer os.Remove(path)

	CreateFile(path, "first")
	CreateFile(path, "second")

	data, _ := os.ReadFile(path)
	if string(data) != "second" {
		t.Errorf("expected 'second' (overwritten), got '%s'", data)
	}
}

func TestCreateFile_Empty(t *testing.T) {
	path := filepath.Join(os.TempDir(), "test_create_empty_"+t.Name()+".txt")
	defer os.Remove(path)

	err := CreateFile(path, "")
	if err != nil {
		t.Fatalf("CreateFile error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Errorf("expected empty file, got size %d", info.Size())
	}
}

// ============================================================
// IsDirEmpty 单元测试
// ============================================================

func TestIsDirEmpty_EmptyDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_empty_dir_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	empty, err := IsDirEmpty(dir)
	if err != nil {
		t.Fatalf("IsDirEmpty error: %v", err)
	}
	if !empty {
		t.Error("should be empty")
	}
}

func TestIsDirEmpty_NonEmptyDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_nonempty_dir_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// 创建文件
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("data"), 0644)

	empty, err := IsDirEmpty(dir)
	if err != nil {
		t.Fatalf("IsDirEmpty error: %v", err)
	}
	if empty {
		t.Error("should not be empty")
	}
}

func TestIsDirEmpty_NotExist(t *testing.T) {
	_, err := IsDirEmpty("/nonexistent_path_12345")
	if err == nil {
		t.Error("expected error for non-existent dir")
	}
}

// ============================================================
// GoFmt 单元测试
// ============================================================

func TestGoFmt_ValidCode(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_gofmt_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.go")
	unformatted := `package main

import "fmt"

func main()    {
fmt.Println(  "hello"  )
}
`
	os.WriteFile(path, []byte(unformatted), 0644)

	err = GoFmt(path)
	if err != nil {
		t.Fatalf("GoFmt error: %v", err)
	}

	// 读取格式化后的内容
	data, _ := os.ReadFile(path)
	content := string(data)
	// 格式化后应包含标准缩进
	if len(content) == 0 {
		t.Error("formatted file should not be empty")
	}
}

func TestGoFmt_InvalidCode(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_gofmt_invalid_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "bad.go")
	os.WriteFile(path, []byte("this is not valid go code {{{"), 0644)

	err = GoFmt(path)
	if err == nil {
		t.Error("expected error for invalid Go code")
	}
}

func TestGoFmt_NonExistent(t *testing.T) {
	err := GoFmt("/nonexistent_file.go")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

// ============================================================
// GoFmtDir 单元测试
// ============================================================

func TestGoFmtDir_Basic(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_gofmtdir_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// 创建有效的 Go 文件
	code := `package main
func main()    {   }
`
	os.WriteFile(filepath.Join(dir, "a.go"), []byte(code), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte(code), 0644)
	// 非 Go 文件不受影响
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("not go"), 0644)

	err = GoFmtDir(dir)
	if err != nil {
		t.Fatalf("GoFmtDir error: %v", err)
	}
}

func TestGoFmtDir_NonExistent(t *testing.T) {
	err := GoFmtDir("/nonexistent_dir_12345")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

// ============================================================
// ParseFile 单元测试
// ============================================================

func TestParseFile_Basic(t *testing.T) {
	header := createFileHeader("test.png", 1024)
	ext, fileName, size := ParseFile(header)

	if ext != "png" {
		t.Errorf("expected ext 'png', got '%s'", ext)
	}
	if fileName != "test" {
		t.Errorf("expected fileName 'test', got '%s'", fileName)
	}
	if size != 1024 {
		t.Errorf("expected size 1024, got %d", size)
	}
}

func TestParseFile_NoExt(t *testing.T) {
	header := createFileHeader("readme", 512)
	ext, _, _ := ParseFile(header)

	if ext != "" {
		t.Errorf("expected empty ext, got '%s'", ext)
	}
}

func TestParseFile_MultiDot(t *testing.T) {
	header := createFileHeader("archive.tar.gz", 2048)
	ext, _, _ := ParseFile(header)

	if ext != "gz" {
		t.Errorf("expected ext 'gz', got '%s'", ext)
	}
}

// ============================================================
// 基准测试
// ============================================================

func BenchmarkExists(b *testing.B) {
	f, _ := os.CreateTemp("", "bench_exists_*")
	f.Close()
	defer os.Remove(f.Name())

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Exists(f.Name())
	}
}

func BenchmarkCreateFile(b *testing.B) {
	dir, _ := os.MkdirTemp("", "bench_create_*")
	defer os.RemoveAll(dir)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		path := filepath.Join(dir, "test.txt")
		CreateFile(path, "benchmark content")
	}
}

func BenchmarkMkdir(b *testing.B) {
	baseDir, _ := os.MkdirTemp("", "bench_mkdir_*")
	defer os.RemoveAll(baseDir)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		dir := filepath.Join(baseDir, fmt.Sprintf("d%d", i))
		Mkdir(dir)
	}
}

func BenchmarkIsDirEmpty(b *testing.B) {
	dir, _ := os.MkdirTemp("", "bench_isempty_*")
	defer os.RemoveAll(dir)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		IsDirEmpty(dir)
	}
}

func BenchmarkParseFile(b *testing.B) {
	header := createFileHeader("test.png", 1024)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ParseFile(header)
	}
}

func BenchmarkGoFmt(b *testing.B) {
	dir, _ := os.MkdirTemp("", "bench_gofmt_*")
	defer os.RemoveAll(dir)

	code := `package main

import "fmt"

func main()    {
fmt.Println(  "hello"  )
}
`
	path := filepath.Join(dir, "test.go")
	os.WriteFile(path, []byte(code), 0644)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// 每次重写文件保证 GoFmt 有效
		os.WriteFile(path, []byte(code), 0644)
		GoFmt(path)
	}
}

// helper: 创建 multipart.FileHeader
func createFileHeader(filename string, size int64) *multipart.FileHeader {
	return &multipart.FileHeader{
		Filename: filename,
		Size:     size,
	}
}
