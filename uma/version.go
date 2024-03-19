package uma

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const MAJOR_VERSION = 1
const MINOR_VERSION = 0

var backcompatVersions = []string{"0.3"}

var UmaProtocolVersion = fmt.Sprintf("%d.%d", MAJOR_VERSION, MINOR_VERSION)

type UnsupportedVersionError struct {
	UnsupportedVersion     string `json:"unsupportedVersion"`
	SupportedMajorVersions []int  `json:"supportedMajorVersions"`
}

func (e UnsupportedVersionError) Error() string {
	return fmt.Sprintf("unsupported version: %s", e.UnsupportedVersion)
}

func GetSupportedMajorVersionsFromErrorResponseBody(errorResponseBody []byte) ([]int, error) {
	responseJson := make(map[string]string)
	err := json.Unmarshal(errorResponseBody, &responseJson)
	if err != nil {
		return nil, err
	}

	vasp2SupportedMajorVersions := responseJson["supportedMajorVersions"]
	vasp2SupportedMajorVersionsList := strings.Split(vasp2SupportedMajorVersions, ",")
	vasp2SupportedMajorVersionsIntList := make([]int, len(vasp2SupportedMajorVersionsList))
	for i, version := range vasp2SupportedMajorVersionsList {
		versionInt, err := strconv.Atoi(version)
		if err != nil {
			return nil, err
		}
		vasp2SupportedMajorVersionsIntList[i] = versionInt
	}
	return vasp2SupportedMajorVersionsIntList, nil
}

func GetSupportedMajorVersions() map[int]struct{} {
	versions := make(map[int]struct{})
	versions[MAJOR_VERSION] = struct{}{}
	for _, version := range backcompatVersions {
		parsedVersion, err := ParseVersion(version)
		if err != nil {
			continue
		}
		versions[parsedVersion.Major] = struct{}{}
	}

	return versions
}

func GetHighestSupportedVersionForMajorVersion(majorVersion int) *ParsedVersion {
	// Note that this also only supports a single major version for now. If we support more than one major version in
	// the future, we'll need to change this.
	if majorVersion == MAJOR_VERSION {
		parsedVersion, _ := ParseVersion(UmaProtocolVersion)
		return parsedVersion
	}
	for _, version := range backcompatVersions {
		parsedVersion, err := ParseVersion(version)
		if err != nil {
			continue
		}
		if parsedVersion.Major == majorVersion {
			return parsedVersion
		}
	}
	return nil
}

func SelectHighestSupportedVersion(otherVaspSupportedMajorVersions []int) *string {
	var highestVersion *ParsedVersion
	supportedMajorVersions := GetSupportedMajorVersions()
	for _, otherVaspMajorVersion := range otherVaspSupportedMajorVersions {
		_, supportsMajorVersion := supportedMajorVersions[otherVaspMajorVersion]
		if !supportsMajorVersion {
			continue
		}

		if highestVersion == nil {
			highestVersion = GetHighestSupportedVersionForMajorVersion(otherVaspMajorVersion)
			continue
		}
		if otherVaspMajorVersion > highestVersion.Major {
			highestVersion = GetHighestSupportedVersionForMajorVersion(otherVaspMajorVersion)
		}
	}
	if highestVersion == nil {
		return nil
	}
	versionString := highestVersion.String()
	return &versionString
}

func SelectLowerVersion(version1String string, version2String string) (*string, error) {
	version1, err := ParseVersion(version1String)
	if err != nil {
		return nil, err
	}
	version2, err := ParseVersion(version2String)
	if err != nil {
		return nil, err
	}
	if version1.Major > version2.Major || (version1.Major == version2.Major && version1.Minor > version2.Minor) {
		return &version2String, nil
	} else {
		return &version1String, nil
	}
}

func IsVersionSupported(version string) bool {
	parsedVersion, err := ParseVersion(version)

	if err != nil || parsedVersion == nil {
		return false
	}
	_, supportsMajorVersion := GetSupportedMajorVersions()[parsedVersion.Major]
	return supportsMajorVersion
}

type ParsedVersion struct {
	Major int
	Minor int
}

func ParseVersion(version string) (*ParsedVersion, error) {
	var major, minor int
	_, err := fmt.Sscanf(version, "%d.%d", &major, &minor)
	if err != nil {
		return nil, err
	}
	return &ParsedVersion{
		Major: major,
		Minor: minor,
	}, nil
}

func (v *ParsedVersion) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}
