// The shape of an SPDX 2.3 document, as structs rather than as maps.
//
// Structs because the field order of a marshalled struct is the order they are
// declared in, so the same inputs render the same bytes - and a document that
// differs every time it is generated cannot be compared with anything. Maps
// would sort their keys, which is also stable, but says nothing about which
// fields exist.
//
// Only the fields this project fills are here. SPDX has more, and a field
// carrying NOASSERTION everywhere is noise that reads like information.

package legal

type spdxDocument struct {
	SPDXVersion       string             `json:"spdxVersion"`
	DataLicense       string             `json:"dataLicense"`
	SPDXID            string             `json:"SPDXID"`
	Name              string             `json:"name"`
	DocumentNamespace string             `json:"documentNamespace"`
	CreationInfo      spdxCreation       `json:"creationInfo"`
	Packages          []spdxPackage      `json:"packages"`
	Relationships     []spdxRelationship `json:"relationships"`
}

type spdxCreation struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
	Comment  string   `json:"comment,omitempty"`
}

type spdxPackage struct {
	SPDXID           string `json:"SPDXID"`
	Name             string `json:"name"`
	VersionInfo      string `json:"versionInfo"`
	DownloadLocation string `json:"downloadLocation"`

	// False, and deliberately. This describes components rather than the
	// individual files they arrive as, and claiming otherwise would oblige the
	// document to list and checksum every file. A half filled file list reads
	// as a complete one.
	FilesAnalyzed bool `json:"filesAnalyzed"`

	LicenseConcluded string            `json:"licenseConcluded"`
	LicenseDeclared  string            `json:"licenseDeclared"`
	CopyrightText    string            `json:"copyrightText"`
	ExternalRefs     []spdxExternalRef `json:"externalRefs,omitempty"`

	// The three below are filled for a companion only - a file the archive
	// carries beside a binary, which has an artefact of its own to name and
	// to checksum, where a module linked into ours does not.
	PackageFileName string         `json:"packageFileName,omitempty"`
	Checksums       []spdxChecksum `json:"checksums,omitempty"`
	Comment         string         `json:"comment,omitempty"`
}

type spdxChecksum struct {
	Algorithm     string `json:"algorithm"`
	ChecksumValue string `json:"checksumValue"`
}

type spdxExternalRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

type spdxRelationship struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
}
