// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/xorcare/pointer"
	"gopkg.in/yaml.v3"

	"github.com/runfinch/finch/pkg/mocks"
)

func TestDiskLimaConfigApplier_Apply(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		config       *Finch
		path         string
		mockSvc      func(fs afero.Fs, l *mocks.Logger)
		postRunCheck func(t *testing.T, fs afero.Fs)
		want         error
	}{
		{
			name: "happy path",
			config: &Finch{
				Memory: pointer.String("2GiB"),
				CPUs:   pointer.Int(4),
			},
			path: "/lima.yaml",
			mockSvc: func(fs afero.Fs, l *mocks.Logger) {
				err := afero.WriteFile(fs, "/lima.yaml", []byte("memory: 4GiB\ncpus: 8"), 0o600)
				require.NoError(t, err)
			},
			postRunCheck: func(t *testing.T, fs afero.Fs) {
				buf, err := afero.ReadFile(fs, "/lima.yaml")
				require.NoError(t, err)

				// limayaml.LimaYAML has a required "images" field which will also get marshaled
				require.Equal(t, buf, []byte("images: []\ncpus: 4\nmemory: 2GiB\n"))
			},
			want: nil,
		},
		{
			name:    "lima config file does not exist",
			config:  nil,
			path:    "/lima.yaml",
			mockSvc: func(afs afero.Fs, l *mocks.Logger) {},
			postRunCheck: func(t *testing.T, mFs afero.Fs) {
				_, err := afero.ReadFile(mFs, "/lima.yaml")
				require.Equal(t, err, &fs.PathError{Op: "open", Path: "/lima.yaml", Err: errors.New("file does not exist")})
			},
			want: fmt.Errorf("failed to load the lima config file: %w",
				&fs.PathError{Op: "open", Path: "/lima.yaml", Err: errors.New("file does not exist")},
			),
		},
		{
			name:   "lima config file does not contain valid YAML",
			config: nil,
			path:   "/lima.yaml",
			mockSvc: func(fs afero.Fs, l *mocks.Logger) {
				err := afero.WriteFile(fs, "/lima.yaml", []byte("this isn't YAML"), 0o600)
				require.NoError(t, err)
			},
			postRunCheck: func(t *testing.T, fs afero.Fs) {
				buf, err := afero.ReadFile(fs, "/lima.yaml")
				require.NoError(t, err)

				require.Equal(t, buf, []byte("this isn't YAML"))
			},
			want: fmt.Errorf(
				"failed to unmarshal the lima config file: %w",
				&yaml.TypeError{Errors: []string{"line 1: cannot unmarshal !!str `this is...` into limayaml.LimaYAML"}},
			),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			l := mocks.NewLogger(ctrl)
			fs := afero.NewMemMapFs()

			tc.mockSvc(fs, l)
			got := NewLimaApplier(tc.config, fs, tc.path).Apply()

			require.Equal(t, tc.want, got)
			tc.postRunCheck(t, fs)
		})
	}
}
