package common

import (
	"os"
	"path/filepath"

	"github.com/elastic/libbeat/logp"

	"github.com/nranchev/go-libGeoIP"
)

type GeoIp struct {
	Paths []string
}

// TODO(shutej): Don't hard-code unix paths.  This could be a platform-specific
// build variable or a library flag with a default location, what makes sense?
var geoipPaths = []string{
	"/usr/share/GeoIP/GeoIP.dat",
	"/usr/local/var/GeoIP/GeoIP.dat",
}

func LoadGeoIPData(config GeoIp) *libgeo.GeoIP {
	if config.Paths != nil {
		geoipPaths = config.Paths
	}
	if len(geoipPaths) == 0 {
		// disabled
		return nil
	}

	// look for the first existing path
	var geoipPath string
	for _, path := range geoipPaths {
		fi, err := os.Lstat(path)
		if err != nil {
			continue
		}

		if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
			// follow symlink
			geoipPath, err = filepath.EvalSymlinks(path)
			if err != nil {
				logp.Warn("Could not load GeoIP data: %s", err.Error())
				return nil
			}
		} else {
			geoipPath = path
		}
		break
	}

	if len(geoipPath) == 0 {
		logp.Warn("Couldn't load GeoIP database")
		return nil
	}

	geoLite, err := libgeo.Load(geoipPath)
	if err != nil {
		logp.Warn("Could not load GeoIP data: %s", err.Error())
	}

	logp.Info("Loaded GeoIP data from: %s", geoipPath)
	return geoLite
}
