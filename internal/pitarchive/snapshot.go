package pitarchive

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"
)

func validateSnapshotPath(relativePath string) error {
	if relativePath == "" || !fs.ValidPath(relativePath) || path.IsAbs(relativePath) || path.Clean(relativePath) != relativePath || strings.Contains(relativePath, "\\") {
		return errors.New("snapshot path must be a clean relative slash-separated path")
	}
	base := path.Base(relativePath)
	if strings.HasPrefix(base, ".") || strings.Contains(base, ".tmp-") || strings.HasSuffix(base, ".tmp") {
		return errors.New("temporary snapshot paths are not accepted")
	}
	return nil
}

func rejectSymlinkComponents(root *os.Root, relativePath string) error {
	parts := strings.Split(relativePath, "/")
	for i := range parts {
		component := strings.Join(parts[:i+1], "/")
		info, err := root.Lstat(component)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("snapshot path contains a symbolic link")
		}
	}
	return nil
}

func verifySnapshots(archiveRoot string, snapshots []SnapshotReference, maxBytes int64) (SnapshotIntegrityReport, []SnapshotReference, []Failure) {
	report := SnapshotIntegrityReport{StrictVerdict: CheckPass}
	if strings.TrimSpace(archiveRoot) == "" {
		report.StrictVerdict = CheckFail
		return report, nil, []Failure{{Code: ReasonSnapshotPathInvalid, Message: "approved archive root is required"}}
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxSnapshotBytes
	}
	root, err := os.OpenRoot(archiveRoot)
	if err != nil {
		report.StrictVerdict = CheckFail
		return report, nil, []Failure{{Code: ReasonSnapshotPathInvalid, Message: "approved archive root is unreadable"}}
	}
	defer root.Close()

	accepted := make([]SnapshotReference, 0, len(snapshots))
	var failures []Failure
	for _, snapshot := range snapshots {
		failure := Failure{SnapshotID: snapshot.SnapshotID, PartitionKey: snapshot.PartitionKey}
		if err := validateSnapshotPath(snapshot.RelativePath); err != nil {
			failure.Code, failure.Message = ReasonSnapshotPathInvalid, err.Error()
			failures = append(failures, failure)
			continue
		}
		if snapshot.ByteSize > maxBytes {
			failure.Code, failure.Message = ReasonSnapshotTooLarge, "snapshot declared size exceeds bounded read policy"
			failures = append(failures, failure)
			continue
		}
		if err := rejectSymlinkComponents(root, snapshot.RelativePath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				failure.Code, failure.Message = ReasonSnapshotMissing, "snapshot file is missing"
			} else {
				failure.Code, failure.Message = ReasonSnapshotPathInvalid, "snapshot path is invalid or contains a symbolic link"
			}
			failures = append(failures, failure)
			continue
		}
		file, err := root.Open(snapshot.RelativePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				failure.Code, failure.Message = ReasonSnapshotMissing, "snapshot file is missing"
			} else {
				failure.Code, failure.Message = ReasonSnapshotPathInvalid, "snapshot file cannot be opened under approved archive root"
			}
			failures = append(failures, failure)
			continue
		}
		info, statErr := file.Stat()
		if statErr != nil || !info.Mode().IsRegular() {
			_ = file.Close()
			failure.Code, failure.Message = ReasonSnapshotPathInvalid, "snapshot must be a regular file"
			failures = append(failures, failure)
			continue
		}
		if info.Size() > maxBytes {
			_ = file.Close()
			failure.Code, failure.Message = ReasonSnapshotTooLarge, "snapshot file exceeds bounded read policy"
			failures = append(failures, failure)
			continue
		}
		hash := sha256.New()
		count, readErr := io.Copy(hash, io.LimitReader(file, maxBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			failure.Code, failure.Message = ReasonSnapshotHashMismatch, "snapshot file could not be read completely"
			failures = append(failures, failure)
			continue
		}
		if count > maxBytes {
			failure.Code, failure.Message = ReasonSnapshotTooLarge, "snapshot file exceeds bounded read policy"
			failures = append(failures, failure)
			continue
		}
		if count != snapshot.ByteSize || count != info.Size() {
			failure.Code, failure.Message = ReasonSnapshotSizeMismatch, "snapshot byte size does not match manifest"
			failures = append(failures, failure)
			continue
		}
		actual := "sha256:" + hex.EncodeToString(hash.Sum(nil))
		if !compareDigest(snapshot.ContentHash, actual) {
			failure.Code, failure.Message = ReasonSnapshotHashMismatch, "snapshot content hash does not match manifest"
			failures = append(failures, failure)
			continue
		}
		accepted = append(accepted, snapshot)
	}
	report.VerifiedCount = len(accepted)
	report.RejectedCount = len(snapshots) - len(accepted)
	if report.RejectedCount > 0 {
		report.StrictVerdict = CheckFail
	}
	return report, accepted, failures
}

func verifyAvailability(manifest SnapshotManifest, cutoff, now time.Time) (AvailabilityReport, []Failure) {
	report := AvailabilityReport{PolicyID: manifest.AvailabilityPolicy.PolicyID, EvaluationCutoff: cutoff, StrictVerdict: CheckPass}
	var failures []Failure
	delay := time.Duration(*manifest.AvailabilityPolicy.RequiredPublicationDelaySeconds) * time.Second
	for _, snapshot := range manifest.Snapshots {
		failure := Failure{SnapshotID: snapshot.SnapshotID, PartitionKey: snapshot.PartitionKey}
		rejected := false
		if snapshot.AvailableAt.IsZero() {
			failure.Code, failure.Message = ReasonAvailabilityTimestampMissing, "snapshot source availability timestamp is missing"
			failures = append(failures, failure)
			rejected = true
		} else {
			if snapshot.AvailableAt.After(cutoff) {
				failure.Code, failure.Message = ReasonAvailableAfterEvaluation, "snapshot became available after evaluation cutoff"
				failures = append(failures, failure)
				rejected = true
			}
			if snapshot.AvailableAt.Before(snapshot.EventTimeEnd.Add(delay)) {
				failure.Code, failure.Message = ReasonPublicationDelayViolation, "snapshot availability precedes required publication delay"
				failures = append(failures, failure)
				rejected = true
			}
		}
		if snapshot.EventTimeStart.After(now) || snapshot.EventTimeEnd.After(now) || snapshot.AvailableAt.After(now) {
			failure.Code, failure.Message = ReasonFutureSnapshotTimestamp, "snapshot contains a future-dated event or availability timestamp"
			failures = append(failures, failure)
			rejected = true
		}
		if rejected {
			report.RejectedCount++
		} else {
			report.AcceptedCount++
		}
	}
	if report.RejectedCount > 0 {
		report.StrictVerdict = CheckFail
	}
	return report, failures
}

func snapshotFailureMessage(snapshot SnapshotReference, format string, args ...any) Failure {
	return Failure{Code: ReasonSnapshotHashMismatch, Message: fmt.Sprintf(format, args...), SnapshotID: snapshot.SnapshotID, PartitionKey: snapshot.PartitionKey}
}
