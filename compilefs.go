package scrapekdl

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
)

// CompileFS compiles a KDL document from fsys and resolves imports within the
// same filesystem using slash-separated io/fs paths.
func CompileFS(ctx context.Context, fsys fs.FS, path string) (*Program, Diagnostics, error) {
	if ctx == nil {
		panic("scrapekdl: nil context")
	}
	if fsys == nil {
		return nil, nil, errors.New("scrapekdl: nil filesystem")
	}
	data, err := readFSFile(ctx, fsys, path)
	if err != nil {
		return nil, nil, err
	}
	return Compile(ctx, Source{Path: path, Data: data}, CompileOptions{
		Loader: func(ctx context.Context, importPath string) ([]byte, error) {
			return readFSFile(ctx, fsys, importPath)
		},
	})
}

// ValidateFS validates a KDL document and its imports from fsys without
// retaining the compiled Program.
func ValidateFS(ctx context.Context, fsys fs.FS, path string) (Diagnostics, error) {
	_, diagnostics, err := CompileFS(ctx, fsys, path)
	return diagnostics, err
}

func readFSFile(ctx context.Context, fsys fs.FS, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "." || !fs.ValidPath(path) {
		return nil, invalidFSPathError(path)
	}
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func invalidFSPathError(path string) error {
	return &fs.PathError{Op: "read", Path: path, Err: fmt.Errorf("%w: path must be a valid io/fs file path", fs.ErrInvalid)}
}
