package savecloud

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/jsj/pokectx/internal/adapter"
)

type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	Key           string    `json:"key"`
	SHA256        string    `json:"sha256"`
	SaveCounter   uint32    `json:"save_counter"`
	UploadedAt    time.Time `json:"uploaded_at"`
	Device        string    `json:"device,omitempty"`
}

type Result struct {
	SchemaVersion int       `json:"schema_version"`
	Status        string    `json:"status"`
	Action        string    `json:"action"`
	Path          string    `json:"path,omitempty"`
	BackupPath    string    `json:"backup_path,omitempty"`
	Manifest      *Manifest `json:"manifest,omitempty"`
}

type File struct {
	Path    string
	Data    []byte
	SHA256  string
	Counter uint32
}

func ReadValid(ctx context.Context, path string) (File, error) {
	observation, err := (adapter.SaveFileReader{Path: path}).Observe(ctx)
	if err != nil {
		return File{}, fmt.Errorf("validate Emerald save %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read Emerald save %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return File{Path: path, Data: data, SHA256: hex.EncodeToString(sum[:]), Counter: observation.SaveCounter}, nil
}

func Export(ctx context.Context, source, destination string) (Result, error) {
	file, err := ReadValid(ctx, source)
	if err != nil {
		return Result{}, err
	}
	if err := writeNew(destination, file.Data); err != nil {
		return Result{}, err
	}
	return Result{SchemaVersion: 1, Status: "ok", Action: "exported", Path: destination}, nil
}

func Import(ctx context.Context, source, destination, adapterAddress string) (Result, error) {
	file, err := ReadValid(ctx, source)
	if err != nil {
		return Result{}, err
	}
	if adapterRunning(adapterAddress) {
		return Result{}, errors.New("close mGBA before importing a save")
	}
	var backupPath string
	if current, err := os.ReadFile(destination); err == nil {
		backupDir := filepath.Join(filepath.Dir(destination), ".pokectx-backups")
		if err := os.MkdirAll(backupDir, 0o700); err != nil {
			return Result{}, fmt.Errorf("create backup directory: %w", err)
		}
		backupPath = filepath.Join(backupDir, time.Now().UTC().Format("20060102T150405.000000000Z")+"-"+filepath.Base(destination))
		if err := writeNew(backupPath, current); err != nil {
			return Result{}, fmt.Errorf("back up current save: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, fmt.Errorf("read current save: %w", err)
	}
	if err := writeAtomic(destination, file.Data); err != nil {
		return Result{}, err
	}
	return Result{SchemaVersion: 1, Status: "ok", Action: "imported", Path: destination, BackupPath: backupPath}, nil
}

func DefaultSavePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", "Pokemon Emerald Pokectx", "Pokemon Emerald.sav"), nil
}

func writeNew(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refusing to replace existing file %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check destination %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create save directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".pokectx-import-*")
	if err != nil {
		return fmt.Errorf("create temporary save: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary save: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync temporary save: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("install imported save: %w", err)
	}
	return nil
}

func adapterRunning(address string) bool {
	if address == "" {
		address = "127.0.0.1:8765"
	}
	connection, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
	if err != nil {
		return false
	}
	connection.Close()
	return true
}

type R2 struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewR2(ctx context.Context) (*R2, error) {
	endpoint, bucket := os.Getenv("POKECTX_R2_ENDPOINT"), os.Getenv("POKECTX_R2_BUCKET")
	accessKey, secretKey := os.Getenv("POKECTX_R2_ACCESS_KEY_ID"), os.Getenv("POKECTX_R2_SECRET_ACCESS_KEY")
	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return nil, errors.New("set POKECTX_R2_ENDPOINT, POKECTX_R2_BUCKET, POKECTX_R2_ACCESS_KEY_ID, and POKECTX_R2_SECRET_ACCESS_KEY")
	}
	configuration, err := config.LoadDefaultConfig(ctx, config.WithRegion("auto"), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")))
	if err != nil {
		return nil, fmt.Errorf("configure R2: %w", err)
	}
	client := s3.NewFromConfig(configuration, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(strings.TrimRight(endpoint, "/"))
		options.UsePathStyle = true
	})
	prefix := strings.Trim(os.Getenv("POKECTX_R2_PREFIX"), "/")
	if prefix == "" {
		prefix = "pokectx"
	}
	return &R2{client: client, bucket: bucket, prefix: prefix}, nil
}

func (r *R2) Backup(ctx context.Context, local File) (Result, error) {
	latest, found, err := r.Latest(ctx)
	if err != nil {
		return Result{}, err
	}
	if found {
		if latest.SHA256 == local.SHA256 {
			return Result{SchemaVersion: 1, Status: "ok", Action: "already_backed_up", Manifest: &latest}, nil
		}
		if latest.SaveCounter == local.Counter || counterNewer(latest.SaveCounter, local.Counter) {
			return Result{}, fmt.Errorf("R2 has a conflicting or newer save at counter %d; run save sync", latest.SaveCounter)
		}
	}
	hostname, _ := os.Hostname()
	key := fmt.Sprintf("%s/saves/%010d-%s.sav", r.prefix, local.Counter, local.SHA256[:16])
	if _, err := r.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &r.bucket, Key: &key, Body: bytes.NewReader(local.Data), ContentType: aws.String("application/octet-stream")}); err != nil {
		return Result{}, fmt.Errorf("upload save to R2: %w", err)
	}
	manifest := Manifest{SchemaVersion: 1, Key: key, SHA256: local.SHA256, SaveCounter: local.Counter, UploadedAt: time.Now().UTC(), Device: hostname}
	data, _ := json.Marshal(manifest)
	manifestKey := r.prefix + "/latest.json"
	if _, err := r.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &r.bucket, Key: &manifestKey, Body: strings.NewReader(string(data)), ContentType: aws.String("application/json")}); err != nil {
		return Result{}, fmt.Errorf("update R2 manifest: %w", err)
	}
	return Result{SchemaVersion: 1, Status: "ok", Action: "uploaded", Manifest: &manifest}, nil
}

func (r *R2) Latest(ctx context.Context) (Manifest, bool, error) {
	key := r.prefix + "/latest.json"
	output, err := r.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &r.bucket, Key: &key})
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) && (apiError.ErrorCode() == "NoSuchKey" || apiError.ErrorCode() == "NotFound") {
			return Manifest{}, false, nil
		}
		return Manifest{}, false, fmt.Errorf("read R2 manifest: %w", err)
	}
	defer output.Body.Close()
	var manifest Manifest
	if err := json.NewDecoder(io.LimitReader(output.Body, 64*1024)).Decode(&manifest); err != nil {
		return Manifest{}, false, fmt.Errorf("decode R2 manifest: %w", err)
	}
	return manifest, true, nil
}

func (r *R2) Download(ctx context.Context, manifest Manifest, destination, adapterAddress string) (Result, error) {
	output, err := r.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &r.bucket, Key: &manifest.Key})
	if err != nil {
		return Result{}, fmt.Errorf("download save from R2: %w", err)
	}
	defer output.Body.Close()
	data, err := io.ReadAll(io.LimitReader(output.Body, 1024*1024))
	if err != nil {
		return Result{}, fmt.Errorf("read R2 save: %w", err)
	}
	temporary, err := os.CreateTemp("", "pokectx-r2-*.sav")
	if err != nil {
		return Result{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return Result{}, err
	}
	temporary.Close()
	file, err := ReadValid(ctx, temporaryPath)
	if err != nil {
		return Result{}, err
	}
	if file.SHA256 != manifest.SHA256 || file.Counter != manifest.SaveCounter {
		return Result{}, errors.New("downloaded R2 save does not match its manifest")
	}
	result, err := Import(ctx, temporaryPath, destination, adapterAddress)
	if err != nil {
		return Result{}, err
	}
	result.Action, result.Manifest = "downloaded", &manifest
	return result, nil
}

func (r *R2) Sync(ctx context.Context, localPath, adapterAddress string) (Result, error) {
	remote, found, err := r.Latest(ctx)
	if err != nil {
		return Result{}, err
	}
	local, localErr := ReadValid(ctx, localPath)
	if localErr != nil {
		if found {
			return r.Download(ctx, remote, localPath, adapterAddress)
		}
		return Result{}, localErr
	}
	if !found || counterNewer(local.Counter, remote.SaveCounter) {
		return r.Backup(ctx, local)
	}
	if local.SHA256 == remote.SHA256 {
		return Result{SchemaVersion: 1, Status: "ok", Action: "current", Manifest: &remote}, nil
	}
	if counterNewer(remote.SaveCounter, local.Counter) {
		return r.Download(ctx, remote, localPath, adapterAddress)
	}
	return Result{}, fmt.Errorf("save conflict: local and R2 both use counter %d with different contents", local.Counter)
}

func counterNewer(candidate, current uint32) bool { return int32(candidate-current) > 0 }
