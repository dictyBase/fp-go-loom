package fs_test

import (
	iofs "io/fs"
	"os"
	"path/filepath"
	"testing"

	F "github.com/IBM/fp-go/v2/function"
	P "github.com/IBM/fp-go/v2/predicate"
	predfs "github.com/dictyBase/fp-go-loom/predicate/fs"
	"github.com/stretchr/testify/require"
)

// stat wraps a FileInfo to exercise the generic extractor shape.
type stat struct {
	info iofs.FileInfo
}

func fileAndDirInfos(t *testing.T) (file, dir iofs.FileInfo) {
	t.Helper()
	dirPath := t.TempDir()
	filePath := filepath.Join(dirPath, "f.txt")
	require.NoError(
		t,
		os.WriteFile(filePath, []byte("x"), 0o600),
	)
	file, err := os.Stat(filePath)
	require.NoError(t, err)
	dir, err = os.Stat(dirPath)
	require.NoError(t, err)
	return file, dir
}

func TestIsDirInfo(t *testing.T) {
	file, dir := fileAndDirInfos(t)
	require.False(t, predfs.IsDirInfo(file))
	require.True(t, predfs.IsDirInfo(dir))
}

func TestIsRegularInfo(t *testing.T) {
	file, dir := fileAndDirInfos(t)
	require.True(t, predfs.IsRegularInfo(file))
	require.False(t, predfs.IsRegularInfo(dir))
}

// TestIsDirComposed exercises the documented composition path:
// atomic predicate + P.ContraMap + any extractor (here a plain
// field lambda, no lens).
func TestIsDirComposed(t *testing.T) {
	file, dir := fileAndDirInfos(t)
	pred := F.Pipe1(
		predfs.IsDirInfo,
		P.ContraMap(func(s stat) iofs.FileInfo {
			return s.info
		}),
	)
	require.False(t, pred(stat{info: file}))
	require.True(t, pred(stat{info: dir}))
}

func TestIsDir(t *testing.T) {
	file, dir := fileAndDirInfos(t)
	pred := predfs.IsDir(func(s stat) iofs.FileInfo {
		return s.info
	})
	require.False(t, pred(stat{info: file}))
	require.True(t, pred(stat{info: dir}))
}

func TestIsRegular(t *testing.T) {
	file, dir := fileAndDirInfos(t)
	pred := predfs.IsRegular(func(s stat) iofs.FileInfo {
		return s.info
	})
	require.True(t, pred(stat{info: file}))
	require.False(t, pred(stat{info: dir}))
}
