package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func main() {
	showGA := flag.Bool("show-ga", false, "Show GA Releases")
	showBeta := flag.Bool("show-beta", false, "Show Beta Releases")
	showRC := flag.Bool("show-rc", false, "Show RC Releases")
	flag.Parse()

	gittag, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	releases, err := MakeReleases(gittag)
	if err != nil {
		log.Fatal(err)
	}
	releases.SetDurations()

	fmt.Print(releases.CSV(*showGA, *showBeta, *showRC))
}

type (
	// Version is the major and minor version number, such as "1.8".
	Version string
	// ReleaseType type is the type of release, such as "rc", "beta" or "."
	ReleaseType string // rc, beta or .
)

const (
	// BetaRelease is a beta.
	BetaRelease ReleaseType = "beta"
	// RCRelease is a release candidate.
	RCRelease = "rc"
	// GARelease is a general availability release.
	GARelease = "."
)

// A Release is the time when a release occurred.
type Release struct {
	date     time.Time
	duration time.Duration
}

// Releases holds all the releases for all versions for all release types.
type Releases map[Version]map[ReleaseType][]Release

// MakeReleases reads out and returns all releases or an error.
//
// out format is expected to be in chronological order, containing the release
// tag and date separated with a tab.
//
// refs/tags/go1.7beta1	Thu Jun 2 10:00:23 2016 +1000
// refs/tags/go1.7beta2	Thu Jun 16 15:41:33 2016 -0400
// refs/tags/go1.7rc1	Thu Jul 7 16:41:29 2016 -0700
// refs/tags/go1.7rc2	Mon Jul 18 08:19:17 2016 -0700
// refs/tags/go1.7	Mon Aug 15 14:09:32 2016 -0700
// refs/tags/go1.7.1	Wed Sep 7 12:11:12 2016 -0700
// refs/tags/go1.7.2	Mon Oct 17 13:43:23 2016 -0700
// refs/tags/go1.7.3	Tue Oct 18 17:02:28 2016 -0700
//
// This output can be obtained with: git tag --format '%(refname),%(authordate)' --sort=authordate
//
func MakeReleases(out []byte) (Releases, error) {
	// sample: go1.7rc1   Thu Jul 7 16:41:29 2016 -0700
	// go versions: go1.8 or go1.8beta1 or go1.9rc1 or go1.8.1
	tags := regexp.MustCompile(`go([0-9]+\.[0-9]+)(\.|rc|beta|)([0-9+]|)\t(.*)`+"\n").FindAllStringSubmatch(string(out), -1)

	releases := make(Releases)
	for _, tag := range tags {
		var (
			version = Version(tag[1])
			revType = ReleaseType(tag[2])
		)

		var num int64
		if tag[3] != "" {
			var err error
			num, err = strconv.ParseInt(tag[3], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("could not parse release number in: %v: %v", tag[0], err)
			}
		}

		date, err := time.Parse("Mon Jan _2 15:04:05 2006 -0700", tag[4])
		if err != nil {
			return nil, fmt.Errorf("could not parse date in: %v: %v", tag[0], err)
		}

		if revType == "" {
			revType = GARelease
		}

		releases.Add(version, revType, int(num), date)
	}
	return releases, nil
}

// Add adds a version, type, number that occurred on date to the releases.
func (r Releases) Add(version Version, typ ReleaseType, num int, date time.Time) {
	if _, ok := r[version]; !ok {
		r[version] = make(map[ReleaseType][]Release)
	}

	if _, ok := r[nextVersion(version)][GARelease]; ok && typ == GARelease {
		// Ignore old GA releases when a newer GA is available, eg, if 1.6
		// has come out and 1.5.4 is also released, ignore the 1.5.4. It's
		// usually just small security patches, and this makes time simple
		// to follow (1.6 marks latest 1.5.x release as the last).
		return
	}

	r[version][typ] = append(r[version][typ], Release{date: date})
}

// SetDurations sets the durations on each release based on when the next
// occurred.
func (r Releases) SetDurations() {
	for version, revs := range r {
		for typ, releases := range revs {
			for i, release := range releases {
				// Set the duration of the last release based on this release's date.
				switch {
				case typ == BetaRelease && i == 0:
					// beta1 is the first release of a new version, don't touch last release.
				case typ == RCRelease && i == 0:
					// rc1 should set the duration of the last beta.
					r.SetLastDuration(version, BetaRelease, release.date)
				case typ == GARelease && i == 0:
					// .0 release should set the duration of the last rc.
					r.SetLastDuration(version, RCRelease, release.date)
					r.SetLastDuration(prevVersion(version), GARelease, release.date)
				default:
					// Could be beta2, rc2, .2 etc
					r.SetDuration(version, typ, release.date, i-1)
				}
			}
		}
	}

	// Set releases that don't have a duration to end today. This allows a user
	// to see where the current release is in comparion to previous releases.
	// This should only affect the latest/current beta or rc and ga.
	for version, revs := range r {
		for typ, releases := range revs {
			for i, release := range releases {
				if release.duration == 0 {
					r.SetDuration(version, typ, time.Now(), i)
				}
			}
		}
	}
}

// SetLastDuration sets the duration of last/current release based on date.
func (r Releases) SetLastDuration(version Version, typ ReleaseType, date time.Time) {
	idx := len(r[version][typ]) - 1
	if idx < 0 {
		return
	}
	r.SetDuration(version, typ, date, idx)
}

// SetDuration sets the duration of the version's revType to be the difference
// between its date and the provided date.
func (r Releases) SetDuration(version Version, typ ReleaseType, date time.Time, idx int) {
	d := date.Sub(r[version][typ][idx].date)
	r[version][typ][idx].duration = d
}

// CSV returns a CSV of the releases.
func (r Releases) CSV(showGA, showBeta, showRC bool) string {
	var (
		buf    bytes.Buffer
		header = []string{""}
	)
	for version, revs := range r {
		for typ, releases := range revs {
			switch {
			case typ == GARelease && !showGA:
				continue
			case typ == BetaRelease && !showBeta:
				continue
			case typ == RCRelease && !showRC:
				continue
			}
			fmt.Fprintf(&buf, "%v%v,", version, typ)
			for i, release := range releases {
				if i > len(header)-2 {
					header = append(header, fmt.Sprintf("%d", i))
				}
				fmt.Fprintf(&buf, "%d,", release.duration/(86400*time.Second))
			}
			fmt.Fprintln(&buf)
		}
	}
	return fmt.Sprintf("%s\n%s", strings.Join(header, ","), buf.String())
}

func nextVersion(current Version) (next Version) {
	major, minor := parseVersion(current)
	return Version(fmt.Sprintf("%d.%d", major, minor+1))
}

func prevVersion(current Version) (previous Version) {
	major, minor := parseVersion(current)
	return Version(fmt.Sprintf("%d.%d", major, minor-1))
}

func parseVersion(version Version) (major int, minor int) {
	v := strings.SplitN(string(version), ".", 2)
	maj, err := strconv.ParseInt(v[0], 10, 64)
	if err != nil {
		panic(err) // passed invalid version string
	}
	min, err := strconv.ParseInt(v[1], 10, 64)
	if err != nil {
		panic(err) // passed invalid version string
	}
	return int(maj), int(min)
}
