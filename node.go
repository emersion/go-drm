package drm

import (
	"fmt"
	"unsafe"
)

type Node struct {
	fd uintptr
}

func NewNode(fd uintptr) *Node {
	return &Node{fd}
}

type Version struct {
	Major, Minor, Patch int32
	Name, Date, Desc    string
}

func (n *Node) Version() (*Version, error) {
	var v versionResp
	if err := version(n.fd, &v); err != nil {
		return nil, err
	}

	name := allocBytes(&v.name, v.nameLen)
	date := allocBytes(&v.date, v.dateLen)
	desc := allocBytes(&v.desc, v.descLen)

	if err := version(n.fd, &v); err != nil {
		return nil, err
	}

	return &Version{
		Major: v.major,
		Minor: v.minor,
		Patch: v.patch,
		Name:  string(name),
		Date:  string(date),
		Desc:  string(desc),
	}, nil
}

type PCIDevice struct {
	Vendor, Device       uint32
	SubVendor, SubDevice uint32
}

func (d *PCIDevice) BusType() BusType {
	return BusPCI
}

type unknownDevice struct {
	busType BusType
}

func (d *unknownDevice) BusType() BusType {
	return d.busType
}

type Device interface {
	BusType() BusType
}

func (n *Node) GetDevice() (Device, error) {
	return n.getDevice()
}

func (n *Node) GetCap(cap Cap) (uint64, error) {
	return getCap(n.fd, uint64(cap))
}

func (n *Node) SetClientCap(cap ClientCap, val uint64) error {
	return setClientCap(n.fd, uint64(cap), val)
}

type ModeCard struct {
	FBs                                      []FBID
	CRTCs                                    []CRTCID
	Connectors                               []ConnectorID
	Encoders                                 []EncoderID
	MinWidth, MaxWidth, MinHeight, MaxHeight uint32
}

func (n *Node) ModeGetResources() (*ModeCard, error) {
	for {
		var r modeCardResp
		if err := modeGetResources(n.fd, &r); err != nil {
			return nil, err
		}
		count := r

		var fbs []FBID
		var crtcs []CRTCID
		var connectors []ConnectorID
		var encoders []EncoderID
		if r.fbsLen > 0 {
			fbs = make([]FBID, r.fbsLen)
			r.fbs = (*uint32)(unsafe.Pointer(&fbs[0]))
		}
		if r.crtcsLen > 0 {
			crtcs = make([]CRTCID, r.crtcsLen)
			r.crtcs = (*uint32)(unsafe.Pointer(&crtcs[0]))
		}
		if r.connectorsLen > 0 {
			connectors = make([]ConnectorID, r.connectorsLen)
			r.connectors = (*uint32)(unsafe.Pointer(&connectors[0]))
		}
		if r.encodersLen > 0 {
			encoders = make([]EncoderID, r.encodersLen)
			r.encoders = (*uint32)(unsafe.Pointer(&encoders[0]))
		}

		if err := modeGetResources(n.fd, &r); err != nil {
			return nil, err
		}

		if r.fbsLen != count.fbsLen || r.crtcsLen != count.crtcsLen || r.connectorsLen != count.connectorsLen || r.encodersLen != count.encodersLen {
			continue
		}

		return &ModeCard{
			FBs:        fbs,
			CRTCs:      crtcs,
			Connectors: connectors,
			Encoders:   encoders,
			MinWidth:   r.minWidth,
			MaxWidth:   r.maxWidth,
			MinHeight:  r.minHeight,
			MaxHeight:  r.maxHeight,
		}, nil
	}
}

func newString(b []byte) string {
	for i := 0; i < len(b); i++ {
		if b[i] == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

type ModeModeInfo struct {
	Clock                                         uint32
	HDisplay, HSyncStart, HSyncEnd, HTotal, HSkew uint16
	VDisplay, VSyncStart, VSyncEnd, VTotal, VScan uint16

	VRefresh uint32

	Flags uint32
	Type  uint32
	Name  string
}

func newModeModeInfo(info *modeModeInfo) *ModeModeInfo {
	return &ModeModeInfo{
		Clock:      info.clock,
		HDisplay:   info.hDisplay,
		HSyncStart: info.hSyncStart,
		HSyncEnd:   info.hSyncEnd,
		HTotal:     info.hTotal,
		HSkew:      info.hSkew,
		VDisplay:   info.vDisplay,
		VSyncStart: info.vSyncStart,
		VSyncEnd:   info.vSyncEnd,
		VTotal:     info.vTotal,
		VScan:      info.vScan,
		VRefresh:   info.vRefresh,
		Flags:      info.flags,
		Type:       info.typ,
		Name:       newString(info.name[:]),
	}
}

func newModeModeInfoList(infos []modeModeInfo) []ModeModeInfo {
	l := make([]ModeModeInfo, len(infos))
	for i, info := range infos {
		l[i] = *newModeModeInfo(&info)
	}
	return l
}

type ModeCRTC struct {
	ID        CRTCID
	FB        FBID
	X, Y      uint32
	GammaSize uint32
	Mode      *ModeModeInfo
}

func (n *Node) ModeGetCRTC(id CRTCID) (*ModeCRTC, error) {
	r := modeCRTCResp{id: uint32(id)}
	if err := modeGetCRTC(n.fd, &r); err != nil {
		return nil, err
	}

	var mode *ModeModeInfo
	if r.modeValid != 0 {
		mode = newModeModeInfo(&r.mode)
	}

	return &ModeCRTC{
		ID:        CRTCID(r.id),
		FB:        FBID(r.fb),
		X:         r.x,
		Y:         r.y,
		GammaSize: r.gammaSize,
		Mode:      mode,
	}, nil
}

type ModeEncoder struct {
	ID                            EncoderID
	Type                          EncoderType
	CRTC                          CRTCID
	PossibleCRTCs, PossibleClones uint32
}

func (n *Node) ModeGetEncoder(id EncoderID) (*ModeEncoder, error) {
	r := modeEncoderResp{id: uint32(id)}
	if err := modeGetEncoder(n.fd, &r); err != nil {
		return nil, err
	}

	return &ModeEncoder{
		ID:             EncoderID(r.id),
		Type:           EncoderType(r.typ),
		CRTC:           CRTCID(r.crtc),
		PossibleCRTCs:  r.possibleCRTCs,
		PossibleClones: r.possibleClones,
	}, nil
}

type ModeConnector struct {
	PossibleEncoders []EncoderID
	Modes            []ModeModeInfo

	Encoder EncoderID
	ID      ConnectorID
	Type    ConnectorType

	Status              ConnectorStatus
	PhyWidth, PhyHeight uint32 // mm
	Subpixel            Subpixel
}

func (n *Node) ModeGetConnector(id ConnectorID) (*ModeConnector, error) {
	for {
		r := modeConnectorResp{id: uint32(id)}
		if err := modeGetConnector(n.fd, &r); err != nil {
			return nil, err
		}
		count := r

		var encoders []EncoderID
		var modes []modeModeInfo
		if r.modesLen > 0 {
			modes = make([]modeModeInfo, r.modesLen)
			r.modes = (*modeModeInfo)(unsafe.Pointer(&modes[0]))
		}
		if r.encodersLen > 0 {
			encoders = make([]EncoderID, r.encodersLen)
			r.encoders = (*uint32)(unsafe.Pointer(&encoders[0]))
		}

		r.propsLen = 0 // don't retrieve properties

		if err := modeGetConnector(n.fd, &r); err != nil {
			return nil, err
		}

		if r.modesLen != count.modesLen || r.encodersLen != count.encodersLen {
			continue
		}

		return &ModeConnector{
			PossibleEncoders: encoders,
			Modes:            newModeModeInfoList(modes),
			Encoder:          EncoderID(r.encoder),
			ID:               ConnectorID(r.id),
			Type:             ConnectorType(r.typ),
			Status:           ConnectorStatus(r.status),
			PhyWidth:         r.phyWidth,
			PhyHeight:        r.phyHeight,
			Subpixel:         Subpixel(r.subpixel),
		}, nil
	}
}

func (n *Node) ModeGetPlaneResources() ([]PlaneID, error) {
	for {
		var r modePlaneResourcesResp
		if err := modeGetPlaneResources(n.fd, &r); err != nil {
			return nil, err
		}
		count := r

		var planes []PlaneID
		if r.planesLen > 0 {
			planes = make([]PlaneID, r.planesLen)
			r.planes = (*uint32)(unsafe.Pointer(&planes[0]))
		}

		if err := modeGetPlaneResources(n.fd, &r); err != nil {
			return nil, err
		}

		if r.planesLen != count.planesLen {
			continue
		}

		return planes, nil
	}
}

type ModePlane struct {
	ID PlaneID

	CRTC CRTCID
	FB   FBID

	PossibleCRTCs uint32
	GammaSize     uint32

	Formats []Format
}

func (n *Node) ModeGetPlane(id PlaneID) (*ModePlane, error) {
	for {
		r := modePlaneResp{id: uint32(id)}
		if err := modeGetPlane(n.fd, &r); err != nil {
			return nil, err
		}
		count := r

		var formats []Format
		if r.formatsLen > 0 {
			formats = make([]Format, r.formatsLen)
			r.formats = (*uint32)(unsafe.Pointer(&formats[0]))
		}

		if err := modeGetPlane(n.fd, &r); err != nil {
			return nil, err
		}

		if r.formatsLen != count.formatsLen {
			continue
		}

		return &ModePlane{
			ID:            PlaneID(r.id),
			CRTC:          CRTCID(r.crtc),
			FB:            FBID(r.fb),
			PossibleCRTCs: r.possibleCRTCs,
			GammaSize:     r.gammaSize,
			Formats:       formats,
		}, nil
	}
}

func (n *Node) ModeObjectGetProperties(id AnyID) (map[PropertyID]uint64, error) {
	for {
		r := modeObjectGetPropertiesResp{
			id:  uint32(id.Object()),
			typ: uint32(id.Type()),
		}
		if err := modeObjectGetProperties(n.fd, &r); err != nil {
			return nil, err
		}
		count := r

		var propIDs []PropertyID
		var propValues []uint64
		if r.propsLen > 0 {
			propIDs = make([]PropertyID, r.propsLen)
			r.propIDs = (*uint32)(unsafe.Pointer(&propIDs[0]))
			propValues = make([]uint64, r.propsLen)
			r.propValues = (*uint64)(unsafe.Pointer(&propValues[0]))
		}

		if err := modeObjectGetProperties(n.fd, &r); err != nil {
			return nil, err
		}

		if r.propsLen != count.propsLen {
			continue
		}

		m := make(map[PropertyID]uint64, r.propsLen)
		for i := 0; i < int(r.propsLen); i++ {
			m[propIDs[i]] = propValues[i]
		}
		return m, nil
	}
}

func (n *Node) ModeGetProperty(id PropertyID) (*ModeProperty, error) {
	r := modeGetPropertyResp{id: uint32(id)}
	if err := modeGetProperty(n.fd, &r); err != nil {
		return nil, err
	}

	var values []uint64
	if r.valuesLen > 0 {
		values = make([]uint64, r.valuesLen)
		r.values = (uintptr)(unsafe.Pointer(&values[0]))
	}

	var enums []modePropertyEnum
	var blobSizes []uint32
	var blobIDs []BlobID
	switch t := newPropertyType(r.flags); t {
	case PropertyEnum, PropertyBitmask:
		if r.enumBlobsLen > 0 {
			enums = make([]modePropertyEnum, r.enumBlobsLen)
			r.enumBlobs = (uintptr)(unsafe.Pointer(&enums[0]))
		}
	case PropertyBlob:
		if r.valuesLen > 0 {
			panic("drm: modeGetPropertyResp.valuesLen > 0 for blob property")
		}
		if r.enumBlobsLen > 0 {
			blobSizes = make([]uint32, r.enumBlobsLen)
			r.values = (uintptr)(unsafe.Pointer(&blobSizes[0]))
			blobIDs = make([]BlobID, r.enumBlobsLen)
			r.enumBlobs = (uintptr)(unsafe.Pointer(&blobIDs[0]))
		}
	default:
		if r.enumBlobsLen > 0 {
			panic(fmt.Sprintf("drm: enumBlobsLen > 0 for %s property", t))
		}
	}

	if err := modeGetProperty(n.fd, &r); err != nil {
		return nil, err
	}

	return &ModeProperty{
		ID:     PropertyID(r.id),
		Name:   newString(r.name[:]),
		flags:  r.flags,
		values: values,
		enums:  newModePropertyEnumList(enums),
		blobs:  newModePropertyBlobList(blobIDs, blobSizes),
	}, nil
}

func (n *Node) ModeGetBlob(id BlobID) ([]byte, error) {
	r := modeGetBlobResp{id: uint32(id)}
	if err := modeGetBlob(n.fd, &r); err != nil {
		return nil, err
	}

	var data []byte
	if r.size > 0 {
		data = make([]byte, r.size)
		r.data = (*byte)(unsafe.Pointer(&data[0]))
	}

	if err := modeGetBlob(n.fd, &r); err != nil {
		return nil, err
	}

	return data, nil
}
