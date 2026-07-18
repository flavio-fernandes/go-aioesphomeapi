// Command protocol-inventory emits and validates the pinned ESPHome compatibility inventory.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type upstream struct {
	Release string `json:"release"`
	Commit  string `json:"commit"`
}

type upstreamLock struct {
	Upstream upstream `json:"upstream"`
}

type versionGate struct {
	Status     string  `json:"status"`
	KnownAt    string  `json:"known_at"`
	MinimumAPI *string `json:"minimum_api"`
}

type evidence struct {
	Known      []string `json:"known"`
	Typed      []string `json:"typed"`
	Simulated  []string `json:"simulated"`
	MGMT       []string `json:"mgmt"`
	Hardware   []string `json:"hardware"`
	Production []string `json:"production"`
}

type unknownValuePlan struct {
	Status   string   `json:"status"`
	Behavior string   `json:"behavior"`
	TestPlan string   `json:"test_plan"`
	Evidence []string `json:"evidence"`
}

type unknownValues struct {
	MessageIDs unknownValuePlan `json:"message_ids"`
	EnumValues unknownValuePlan `json:"enum_values"`
	Fields     unknownValuePlan `json:"fields"`
}

type messageAnnotation struct {
	Name            string       `json:"name"`
	Milestone       string       `json:"milestone"`
	EntityFamily    string       `json:"entity_family"`
	MGMTRequired    bool         `json:"mgmt_required"`
	ReferenceParity string       `json:"reference_parity"`
	PublicBehavior  string       `json:"public_behavior"`
	EvidenceProfile string       `json:"evidence_profile"`
	Notes           string       `json:"notes"`
	VersionGate     *versionGate `json:"version_gate,omitempty"`
}

type annotations struct {
	SchemaVersion      int                 `json:"schema_version"`
	Upstream           upstream            `json:"upstream"`
	DefaultVersionGate versionGate         `json:"default_version_gate"`
	UnknownValues      unknownValues       `json:"unknown_values"`
	EvidenceProfiles   map[string]evidence `json:"evidence_profiles"`
	Messages           []messageAnnotation `json:"messages"`
}

type entry struct {
	ID              uint32      `json:"id"`
	Name            string      `json:"name"`
	Direction       string      `json:"direction"`
	VersionGate     versionGate `json:"version_gate"`
	FeatureGate     string      `json:"feature_gate"`
	EntityFamily    string      `json:"entity_family"`
	Milestone       string      `json:"milestone"`
	MGMTRequired    bool        `json:"mgmt_required"`
	ReferenceParity string      `json:"reference_parity"`
	PublicBehavior  string      `json:"public_behavior"`
	Evidence        evidence    `json:"evidence"`
	Notes           string      `json:"notes"`
}

type inventory struct {
	SchemaVersion int           `json:"schema_version"`
	Upstream      upstream      `json:"upstream"`
	UnknownValues unknownValues `json:"unknown_values"`
	Messages      []entry       `json:"messages"`
}

func extension[T any](options *descriptorpb.MessageOptions, extension protoreflect.ExtensionType, fallback T) T {
	if !proto.HasExtension(options, extension) {
		return fallback
	}
	value, ok := proto.GetExtension(options, extension).(T)
	if !ok {
		return fallback
	}
	return value
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing JSON data")
	}
	return nil
}

func readLock(path string, target *upstreamLock) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// featureFamily derives a machine-readable entity family from an upstream
// ifdef feature gate. A compound gate expression needs an explicit reviewed
// mapping here; an unmapped one must fail generation instead of embedding a
// malformed family name in the published inventory.
func featureFamily(ifdef string) (string, error) {
	if ifdef == "" {
		return "protocol", nil
	}
	if ifdef == "USE_IR_RF || USE_RADIO_FREQUENCY" {
		return "infrared_radio_frequency", nil
	}
	family := strings.ToLower(strings.TrimPrefix(ifdef, "USE_"))
	if !isEntityFamilyName(family) {
		return "", fmt.Errorf("feature gate %q derives invalid entity family %q; add a reviewed mapping in featureFamily", ifdef, family)
	}
	return family, nil
}

func isEntityFamilyName(family string) bool {
	if family == "" {
		return false
	}
	for _, character := range family {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}

func emptyEvidence() evidence {
	return evidence{
		Known:      []string{"protocol/upstream.lock.json", "protocol/upstream/api.proto"},
		Typed:      []string{},
		Simulated:  []string{},
		MGMT:       []string{},
		Hardware:   []string{},
		Production: []string{},
	}
}

func validatePlan(name string, plan unknownValuePlan) error {
	if plan.Status == "" || plan.Behavior == "" || plan.TestPlan == "" || plan.Evidence == nil {
		return fmt.Errorf("unknown-value plan %s must declare status, behavior, test_plan, and evidence", name)
	}
	return nil
}

func validateEvidencePaths(root, owner string, values evidence) error {
	levels := map[string][]string{
		"known": values.Known, "typed": values.Typed, "simulated": values.Simulated,
		"mgmt": values.MGMT, "hardware": values.Hardware, "production": values.Production,
	}
	for level, paths := range levels {
		if paths == nil {
			return fmt.Errorf("evidence profile %s omits %s", owner, level)
		}
		for _, path := range paths {
			if path == "" {
				return fmt.Errorf("evidence profile %s has an empty %s reference", owner, level)
			}
			if strings.HasPrefix(path, "https://") {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				return fmt.Errorf("evidence profile %s references %s: %w", owner, path, err)
			}
		}
	}
	return nil
}

func validatePlanPaths(root, owner string, plan unknownValuePlan) error {
	for _, path := range plan.Evidence {
		if path == "" {
			return fmt.Errorf("unknown-value plan %s has an empty evidence reference", owner)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			return fmt.Errorf("unknown-value plan %s references %s: %w", owner, path, err)
		}
	}
	return nil
}

func buildInventory(annotationPath, lockPath string) (inventory, error) {
	var config annotations
	if err := readJSON(annotationPath, &config); err != nil {
		return inventory{}, fmt.Errorf("read annotations: %w", err)
	}
	var lock upstreamLock
	if err := readLock(lockPath, &lock); err != nil {
		return inventory{}, fmt.Errorf("read upstream lock: %w", err)
	}
	if config.SchemaVersion != 1 {
		return inventory{}, fmt.Errorf("unsupported annotation schema %d", config.SchemaVersion)
	}
	if config.Upstream != lock.Upstream {
		return inventory{}, fmt.Errorf("annotation upstream %s@%s does not match lock %s@%s", config.Upstream.Release, config.Upstream.Commit, lock.Upstream.Release, lock.Upstream.Commit)
	}
	if config.DefaultVersionGate.Status == "" || config.DefaultVersionGate.KnownAt == "" {
		return inventory{}, errors.New("default version gate must declare status and known_at")
	}
	repositoryRoot := filepath.Dir(filepath.Dir(annotationPath))
	for name, plan := range map[string]unknownValuePlan{
		"message_ids": config.UnknownValues.MessageIDs,
		"enum_values": config.UnknownValues.EnumValues,
		"fields":      config.UnknownValues.Fields,
	} {
		if err := validatePlan(name, plan); err != nil {
			return inventory{}, err
		}
		if err := validatePlanPaths(repositoryRoot, name, plan); err != nil {
			return inventory{}, err
		}
	}
	for name, profile := range config.EvidenceProfiles {
		if err := validateEvidencePaths(repositoryRoot, name, profile); err != nil {
			return inventory{}, err
		}
	}

	overrides := make(map[string]messageAnnotation, len(config.Messages))
	for _, annotation := range config.Messages {
		if annotation.Name == "" {
			return inventory{}, errors.New("message annotation has an empty name")
		}
		if _, exists := overrides[annotation.Name]; exists {
			return inventory{}, fmt.Errorf("duplicate annotation for %s", annotation.Name)
		}
		if annotation.Milestone != "M1" {
			return inventory{}, fmt.Errorf("annotation for %s must be an M1 message", annotation.Name)
		}
		if annotation.EntityFamily == "" || annotation.ReferenceParity == "" || annotation.PublicBehavior == "" || annotation.Notes == "" {
			return inventory{}, fmt.Errorf("M1 annotation for %s is incomplete", annotation.Name)
		}
		profile, ok := config.EvidenceProfiles[annotation.EvidenceProfile]
		if !ok {
			return inventory{}, fmt.Errorf("M1 annotation for %s names unknown evidence profile %q", annotation.Name, annotation.EvidenceProfile)
		}
		if annotation.PublicBehavior != "generated_only" && (len(profile.Typed) == 0 || len(profile.Simulated) == 0) {
			return inventory{}, fmt.Errorf("M1 annotation for %s lacks typed or simulated evidence", annotation.Name)
		}
		overrides[annotation.Name] = annotation
	}

	messages := pb.File_api_proto.Messages()
	entries := make([]entry, 0, messages.Len())
	seenIDs := make(map[uint32]string)
	seenNames := make(map[string]struct{})
	for i := 0; i < messages.Len(); i++ {
		descriptor := messages.Get(i)
		options, ok := descriptor.Options().(*descriptorpb.MessageOptions)
		if !ok {
			continue
		}
		id := extension(options, pb.E_Id, uint32(0))
		if id == 0 {
			continue
		}
		name := string(descriptor.Name())
		if previous, exists := seenIDs[id]; exists {
			return inventory{}, fmt.Errorf("duplicate message id %d: %s and %s", id, previous, name)
		}
		seenIDs[id] = name
		seenNames[name] = struct{}{}
		ifdef := extension(options, pb.E_Ifdef, "")
		family, err := featureFamily(ifdef)
		if err != nil {
			return inventory{}, fmt.Errorf("message %s: %w", name, err)
		}
		item := entry{
			ID:              id,
			Name:            name,
			Direction:       extension(options, pb.E_Source, pb.APISourceType_SOURCE_BOTH).String(),
			VersionGate:     config.DefaultVersionGate,
			FeatureGate:     ifdef,
			EntityFamily:    family,
			Milestone:       "not_scheduled",
			MGMTRequired:    false,
			ReferenceParity: "not_assessed",
			PublicBehavior:  "generated_only",
			Evidence:        emptyEvidence(),
			Notes:           "Generated wire type only; no support claim beyond known.",
		}
		if item.FeatureGate == "" {
			item.FeatureGate = "always"
		}
		if annotation, exists := overrides[name]; exists {
			item.Milestone = annotation.Milestone
			item.EntityFamily = annotation.EntityFamily
			item.MGMTRequired = annotation.MGMTRequired
			item.ReferenceParity = annotation.ReferenceParity
			item.PublicBehavior = annotation.PublicBehavior
			item.Evidence = config.EvidenceProfiles[annotation.EvidenceProfile]
			item.Notes = annotation.Notes
			if annotation.VersionGate != nil {
				item.VersionGate = *annotation.VersionGate
			}
			delete(overrides, name)
		}
		entries = append(entries, item)
	}
	if len(overrides) != 0 {
		names := make([]string, 0, len(overrides))
		for name := range overrides {
			names = append(names, name)
		}
		sort.Strings(names)
		return inventory{}, fmt.Errorf("annotations name messages absent from pinned protocol: %s", strings.Join(names, ", "))
	}
	if len(entries) == 0 || len(seenIDs) != len(entries) || len(seenNames) != len(entries) {
		return inventory{}, fmt.Errorf("pinned protocol inventory is empty or non-unique: %d messages", len(entries))
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return inventory{SchemaVersion: 2, Upstream: lock.Upstream, UnknownValues: config.UnknownValues, Messages: entries}, nil
}

func render(value inventory) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func main() {
	annotationPath := flag.String("annotations", "protocol/inventory.annotations.json", "curated compatibility annotations")
	lockPath := flag.String("lock", "protocol/upstream.lock.json", "pinned upstream lock")
	checkPath := flag.String("check", "", "verify that a generated inventory file is current")
	outputPath := flag.String("output", "", "write generated inventory to a file")
	summary := flag.Bool("summary", false, "print a short human-readable summary")
	flag.Parse()

	value, err := buildInventory(*annotationPath, *lockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build inventory: %v\n", err)
		os.Exit(1)
	}
	data, err := render(value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode inventory: %v\n", err)
		os.Exit(1)
	}
	if *checkPath != "" {
		committed, err := os.ReadFile(*checkPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read inventory: %v\n", err)
			os.Exit(1)
		}
		if !bytes.Equal(data, committed) {
			fmt.Fprintf(os.Stderr, "%s is stale; run ./tools/generate-protocol.sh\n", *checkPath)
			os.Exit(1)
		}
		if !*summary {
			fmt.Printf("protocol inventory is current: %d unique messages\n", len(value.Messages))
			return
		}
	}
	if *summary {
		m1 := 0
		implemented := 0
		generatedOnly := 0
		for _, message := range value.Messages {
			if message.Milestone == "M1" {
				m1++
			}
			if message.PublicBehavior == "generated_only" {
				generatedOnly++
			} else {
				implemented++
			}
		}
		fmt.Printf("ESPHome %s: %d unique messages, %d M1 accounted (%d implemented), %d generated-only\n", value.Upstream.Release, len(value.Messages), m1, implemented, generatedOnly)
		return
	}
	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write inventory: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "write inventory: %v\n", err)
		os.Exit(1)
	}
}
