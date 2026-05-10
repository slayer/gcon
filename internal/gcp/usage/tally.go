package usage

import (
	"strings"
	"time"

	"github.com/slayer/gcon/internal/gcp"
)

// rootBucket is the synthetic key used in ByTopPrefix for objects that live
// directly under the scan root (no further "/" separator).
const rootBucket = "(root)"

// noExtension is the synthetic key used in ByExtension for files without one.
const noExtension = "(none)"

// topPrefixSegment returns the first path segment of fullName *relative to*
// scanPrefix. For objects sitting directly under the scan root (no further
// slash), it returns "(root)". The scanPrefix may or may not end with a slash;
// both are accepted.
func topPrefixSegment(fullName, scanPrefix string) string {
	rel := fullName
	if scanPrefix != "" {
		// Normalize: ensure scanPrefix ends with "/" before stripping.
		normalized := scanPrefix
		if !strings.HasSuffix(normalized, "/") {
			normalized += "/"
		}
		rel = strings.TrimPrefix(fullName, normalized)
	}
	if rel == "" {
		return rootBucket
	}
	if i := strings.Index(rel, "/"); i >= 0 {
		return rel[:i+1] // include trailing slash for clarity
	}
	return rootBucket
}

// extensionOf returns the lowercase extension of the basename (including the
// leading dot), or "(none)" if there isn't one. Hidden files starting with "."
// and no further dot (e.g. ".bashrc") count as having no extension.
func extensionOf(fullName string) string {
	base := fullName
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if base == "" {
		return noExtension
	}
	// Strip a leading dot for hidden files so ".bashrc" → "bashrc" → no ext,
	// while ".env.local" → ".local".
	stripped := strings.TrimPrefix(base, ".")
	dot := strings.LastIndex(stripped, ".")
	if dot < 0 {
		return noExtension
	}
	return strings.ToLower(stripped[dot:])
}

// tallyObjects walks the objects and produces a fully populated BucketUsage
// with breakdowns. The classes slice MUST be the same length as objs — entry i
// is the storage class of object i. Pass an empty string to skip classification
// for that object.
func tallyObjects(bucket, prefix string, objs []gcp.StorageObject, classes []string) BucketUsage {
	now := time.Now()
	u := BucketUsage{
		Bucket:         bucket,
		Prefix:         prefix,
		ByStorageClass: make(map[string]Stat),
		ByTopPrefix:    make(map[string]Stat),
		ByExtension:    make(map[string]Stat),
		Source:         SourceDeepScan,
		ScannedAt:      now,
		// For deep scans, the data freshness equals the scan time.
		AsOf: now,
	}
	for i, o := range objs {
		if o.IsFolder {
			continue // virtual prefix, not a real object
		}
		u.TotalBytes += o.Size
		u.ObjectCount++
		if i < len(classes) && classes[i] != "" {
			cur := u.ByStorageClass[classes[i]]
			cur.Bytes += o.Size
			cur.Count++
			u.ByStorageClass[classes[i]] = cur
		}
		seg := topPrefixSegment(o.Name, prefix)
		curP := u.ByTopPrefix[seg]
		curP.Bytes += o.Size
		curP.Count++
		u.ByTopPrefix[seg] = curP
		ext := extensionOf(o.Name)
		curE := u.ByExtension[ext]
		curE.Bytes += o.Size
		curE.Count++
		u.ByExtension[ext] = curE
	}
	return u
}
