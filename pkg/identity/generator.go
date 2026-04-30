package identity

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GeneratorOptions configures how a new profile is generated.
type GeneratorOptions struct {
	// Name is an optional human-readable label for the generated profile.
	// If empty a name is auto-generated.
	Name string

	// Template seeds the generator with a starting configuration.
	// If nil, a default Windows 11 desktop template is used.
	Template *Profile

	// Seed is the random seed. If 0, a time-based seed is used.
	Seed int64
}

// Generator produces randomised but realistic profiles.
// It is not safe for concurrent use; create one per goroutine.
type Generator struct {
	rng *rand.Rand
}

// NewGenerator constructs a Generator with the given seed.
// A seed of 0 means "use current time".
func NewGenerator(seed int64) *Generator {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &Generator{rng: rand.New(rand.NewSource(seed))} //nolint:gosec
}

// Generate creates a new profile using the given options.
// If opts is nil, defaults are applied.
func (g *Generator) Generate(opts *GeneratorOptions) *Profile {
	if opts == nil {
		opts = &GeneratorOptions{}
	}

	base := Windows11DesktopTemplate()
	if opts.Template != nil {
		// Clone template as base so we only randomise fields not explicitly set.
		clone := *opts.Template
		base = &clone
	}

	now := time.Now().UTC()
	base.ID = uuid.New().String()
	base.SchemaVersion = CurrentSchemaVersion
	base.CreatedAt = now
	base.UpdatedAt = now

	if opts.Name != "" {
		base.Name = opts.Name
	} else {
		base.Name = g.randomName()
	}

	// Randomise hardware within realistic Windows 11 ranges.
	base.Hardware.CPUCores = g.pickFrom([]int{4, 6, 8, 12, 16})
	base.Hardware.RAMMb = g.pickFrom([]int{4096, 8192, 16384, 32768})
	base.Hardware.GPUVendor, base.Hardware.GPURenderer = g.randomGPU()

	// Randomise screen geometry from common resolutions.
	base.Screen = g.randomScreen()

	// Randomise noise seeds independently.
	base.Noise.CanvasSeed = g.rng.Int63()
	base.Noise.AudioSeed = g.rng.Int63()
	base.Noise.WebGLSeed = g.rng.Int63()
	base.Noise.FontSeed = g.rng.Int63()

	// Randomise User-Agent Chrome minor version.
	base.Browser.UserAgent, base.Browser.AppVersion = g.randomChromeUA()

	return base
}

// GenerateN generates count unique profiles, guaranteeing unique IDs and noise
// seeds across the batch.
func (g *Generator) GenerateN(count int, opts *GeneratorOptions) []*Profile {
	out := make([]*Profile, count)
	for i := range out {
		out[i] = g.Generate(opts)
	}
	return out
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (g *Generator) pickFrom(choices []int) int {
	return choices[g.rng.Intn(len(choices))]
}

func (g *Generator) randomName() string {
	adjectives := []string{"swift", "quiet", "bright", "steady", "calm", "spare"}
	nouns := []string{"falcon", "cedar", "harbor", "ridge", "stone", "creek"}
	adj := adjectives[g.rng.Intn(len(adjectives))]
	noun := nouns[g.rng.Intn(len(nouns))]
	num := g.rng.Intn(900) + 100
	return fmt.Sprintf("%s-%s-%d", adj, noun, num)
}

type gpuEntry struct{ vendor, renderer string }

var gpuPool = []gpuEntry{
	{
		"Google Inc. (NVIDIA)",
		"ANGLE (NVIDIA, NVIDIA GeForce GTX 1660 SUPER Direct3D11 vs_5_0 ps_5_0, D3D11)",
	},
	{
		"Google Inc. (NVIDIA)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	},
	{
		"Google Inc. (NVIDIA)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3070 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	},
	{
		"Google Inc. (NVIDIA)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4070 Direct3D12 vs_5_0 ps_5_0, D3D12)",
	},
	{
		"Google Inc. (AMD)",
		"ANGLE (AMD, AMD Radeon RX 6600 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	},
	{
		"Google Inc. (AMD)",
		"ANGLE (AMD, AMD Radeon RX 7600 Direct3D12 vs_5_0 ps_5_0, D3D12)",
	},
	{
		"Google Inc. (Intel)",
		"ANGLE (Intel, Intel(R) UHD Graphics 770 Direct3D11 vs_5_0 ps_5_0, D3D11)",
	},
}

func (g *Generator) randomGPU() (vendor, renderer string) {
	e := gpuPool[g.rng.Intn(len(gpuPool))]
	return e.vendor, e.renderer
}

type screenResolution struct {
	w, h        int
	availH      int
	dpr         float64
	orientation Orientation
}

var screenPool = []screenResolution{
	{1920, 1080, 1040, 1.0, OrientationLandscapePrimary},
	{2560, 1440, 1400, 1.0, OrientationLandscapePrimary},
	{1920, 1200, 1160, 1.0, OrientationLandscapePrimary},
	{2560, 1080, 1040, 1.0, OrientationLandscapePrimary},
	{3440, 1440, 1400, 1.0, OrientationLandscapePrimary},
	{1280, 800, 760, 2.0, OrientationLandscapePrimary},
	{1366, 768, 728, 1.0, OrientationLandscapePrimary},
	{1920, 1080, 1040, 1.25, OrientationLandscapePrimary},
	{2560, 1600, 1560, 2.0, OrientationLandscapePrimary},
}

func (g *Generator) randomScreen() ScreenProfile {
	r := screenPool[g.rng.Intn(len(screenPool))]
	return ScreenProfile{
		Width:            r.w,
		Height:           r.h,
		AvailWidth:       r.w,
		AvailHeight:      r.availH,
		ColorDepth:       24,
		PixelDepth:       24,
		DevicePixelRatio: r.dpr,
		Orientation:      r.orientation,
	}
}

// chromeMajorVersions lists recent stable Chrome major versions.
var chromeMajorVersions = []int{120, 121, 122, 123, 124, 125}

func (g *Generator) randomChromeUA() (ua, appVersion string) {
	major := chromeMajorVersions[g.rng.Intn(len(chromeMajorVersions))]
	minor := g.rng.Intn(10)
	build := g.rng.Intn(9000) + 1000
	patch := g.rng.Intn(100)

	version := fmt.Sprintf("%d.%d.%d.%d", major, minor, build, patch)
	core := fmt.Sprintf("Windows NT 10.0; Win64; x64") +
		fmt.Sprintf(") AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version)
	ua = "Mozilla/5.0 (" + core
	appVersion = "5.0 (" + strings.TrimPrefix(ua, "Mozilla/5.0 (")
	return ua, appVersion
}
