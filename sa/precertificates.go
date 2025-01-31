package sa

import (
	"crypto/x509"
	"errors"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/letsencrypt/boulder/core"
	corepb "github.com/letsencrypt/boulder/core/proto"
	berrors "github.com/letsencrypt/boulder/errors"
	sapb "github.com/letsencrypt/boulder/sa/proto"
)

var errIncompleteRequest = errors.New("Incomplete gRPC request message")

// AddSerial writes a record of a serial number generation to the DB.
func (ssa *SQLStorageAuthority) AddSerial(ctx context.Context, req *sapb.AddSerialRequest) (*corepb.Empty, error) {
	if req == nil || req.Created == nil || req.Expires == nil || req.Serial == nil || req.RegID == nil {
		return nil, errIncompleteRequest
	}
	created := time.Unix(0, *req.Created)
	expires := time.Unix(0, *req.Expires)
	err := ssa.dbMap.WithContext(ctx).Insert(&recordedSerialModel{
		Serial:         *req.Serial,
		RegistrationID: *req.RegID,
		Created:        created,
		Expires:        expires,
	})
	if err != nil {
		return nil, err
	}
	return &corepb.Empty{}, nil
}

// AddPrecertificate writes a record of a precertificate generation to the DB.
func (ssa *SQLStorageAuthority) AddPrecertificate(ctx context.Context, req *sapb.AddCertificateRequest) (*corepb.Empty, error) {
	if req == nil || req.Der == nil || req.Issued == nil || req.RegID == nil {
		return nil, errIncompleteRequest
	}
	parsed, err := x509.ParseCertificate(req.Der)
	if err != nil {
		return nil, err
	}
	issued := time.Unix(0, *req.Issued)
	serialHex := core.SerialToString(parsed.SerialNumber)
	err = ssa.dbMap.WithContext(ctx).Insert(&precertificateModel{
		Serial:         serialHex,
		RegistrationID: *req.RegID,
		DER:            req.Der,
		Issued:         issued,
		Expires:        parsed.NotAfter,
	})
	if err != nil {
		if strings.HasPrefix(err.Error(), "Error 1062: Duplicate entry") {
			return nil, berrors.DuplicateError("cannot add a duplicate precertificate")
		}
		return nil, err
	}

	err = ssa.dbMap.WithContext(ctx).Insert(&certStatusModel{
		Status:          core.OCSPStatusGood,
		OCSPLastUpdated: ssa.clk.Now(),
		OCSPResponse:    req.Ocsp,
		Serial:          serialHex,
		RevokedDate:     time.Time{},
		RevokedReason:   0,
		NotAfter:        parsed.NotAfter,
	})
	if err != nil {
		return nil, err
	}
	return &corepb.Empty{}, nil
}
