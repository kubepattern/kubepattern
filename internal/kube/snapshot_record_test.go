//go:build snapshot

package kube

// Records a cluster snapshot for TestSnapshotGroundTruth, through the engine's own fetch
// path: every Pattern of the snapshot's suites (original and v2) is fetched with
// FetchForPattern on the live cluster, and the discovery documents and the fetched objects
// are written to testdata/replication/snapshot/<context>.json.gz. The clusters kp-eval and
// kp-krateo are built with the scripts of the replication package
// (https://github.com/kubepattern/kubepattern-replication-2026), which keeps its own copy of
// the snapshots in measurements/limits/snapshot/.
//
//	KP_SNAPSHOT=kp-eval go test -tags snapshot -run TestRecordSnapshot ./internal/kube/
//
// The current kubeconfig context must be the snapshot's name. Objects are trimmed of
// managedFields and last-applied annotations, and Secrets of their data.

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"

	"testing"
)

func TestRecordSnapshot(t *testing.T) {
	name := os.Getenv("KP_SNAPSHOT")
	if name == "" {
		t.Skip("set KP_SNAPSHOT to the kubeconfig context to record (kp-eval or kp-krateo)")
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	raw, err := rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	if raw.CurrentContext != name {
		t.Fatalf("current context is %q, want %q", raw.CurrentContext, name)
	}
	cfg, err := clientcmd.NewDefaultClientConfig(*raw, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.QPS, cfg.Burst = 50, 100
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Every suite of the snapshot (original and v2 Patterns share names, so each suite is
	// fetched with its own client cache); the snapshot is the union of what they read.
	ctx := context.Background()
	objects := map[string]snapshotObject{}
	for _, s := range evalSuites(t) {
		if s.snapshot != name {
			continue
		}
		fc, err := NewClient(cfg)
		if err != nil {
			t.Fatal(err)
		}
		var all []unstructured.Unstructured
		for _, p := range loadPatterns(t, s.patternDirs...) {
			res, err := fc.FetchForPattern(ctx, p, all)
			all = append(all, res...)
			if err != nil {
				t.Errorf("%s: %s: %v", s.name, p.Metadata.Name, err)
			}
		}
		for i := range all {
			o := all[i].DeepCopy()
			gvk := o.GroupVersionKind()
			gvr, err := c.GetGVR(gvk)
			if err != nil {
				t.Fatalf("%s: %v", gvk, err)
			}
			trim(o)
			key := gvr.String() + "|" + o.GetNamespace() + "|" + o.GetName()
			objects[key] = snapshotObject{Group: gvr.Group, Version: gvr.Version, Resource: gvr.Resource, Object: o.Object}
		}
	}

	groups, lists, err := c.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		t.Logf("partial discovery: %v", err)
	}
	preferredGV := map[string]string{}
	for _, g := range groups {
		preferredGV[g.Name] = g.PreferredVersion.GroupVersion
	}
	// The fake discovery takes the first list of a group as its preferred version.
	sort.SliceStable(lists, func(i, j int) bool {
		gi, _ := schema.ParseGroupVersion(lists[i].GroupVersion)
		gj, _ := schema.ParseGroupVersion(lists[j].GroupVersion)
		if gi.Group != gj.Group {
			return gi.Group < gj.Group
		}
		return lists[i].GroupVersion == preferredGV[gi.Group] && lists[j].GroupVersion != preferredGV[gj.Group]
	})
	preferred, err := c.discoveryClient.ServerPreferredResources()
	if err != nil {
		t.Logf("partial preferred discovery: %v", err)
	}

	snap := snapshot{Context: name, Recorded: time.Now().UTC().Format(time.RFC3339), Engine: "FetchForPattern",
		Resources: trimLists(lists), Preferred: trimLists(preferred)}
	keys := make([]string, 0, len(objects))
	for k := range objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		snap.Objects = append(snap.Objects, objects[k])
	}

	out := snapshotPath(name)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	enc := json.NewEncoder(gz)
	enc.SetIndent("", " ")
	if err := enc.Encode(snap); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("recorded %d objects and %d group versions to %s", len(snap.Objects), len(snap.Resources), out)
}

// trim drops what the engine never reads and what must not be published.
func trim(o *unstructured.Unstructured) {
	o.SetManagedFields(nil)
	ann := o.GetAnnotations()
	delete(ann, "kubectl.kubernetes.io/last-applied-configuration")
	if len(ann) == 0 {
		ann = nil
	}
	o.SetAnnotations(ann)
	if o.GetKind() == "Secret" && o.GetAPIVersion() == "v1" {
		delete(o.Object, "data")
		delete(o.Object, "stringData")
	}
}

// trimLists keeps the fields of the discovery documents that the client reads.
func trimLists(lists []*metav1.APIResourceList) []*metav1.APIResourceList {
	var out []*metav1.APIResourceList
	for _, l := range lists {
		if l == nil {
			continue
		}
		nl := &metav1.APIResourceList{GroupVersion: l.GroupVersion}
		for _, r := range l.APIResources {
			nl.APIResources = append(nl.APIResources, metav1.APIResource{
				Name: r.Name, Kind: r.Kind, Namespaced: r.Namespaced, Verbs: r.Verbs, Categories: r.Categories,
				SingularName: r.SingularName, ShortNames: r.ShortNames,
			})
		}
		out = append(out, nl)
	}
	return out
}
