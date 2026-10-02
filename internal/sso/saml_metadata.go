package sso

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"strings"

	"github.com/crewjam/saml"
	xrv "github.com/mattermost/xml-roundtrip-validator"
	"github.com/pkg/errors"
)

// parseIdPMetadata parses the IdP metadata, the root element can also be EntitiesDescriptor.
func parseIdPMetadata(data []byte) (*saml.EntityDescriptor, error) {
	if err := xrv.Validate(bytes.NewReader(data)); err != nil {
		return nil, errors.Wrap(err, "validate XML")
	}
	entity := &saml.EntityDescriptor{}
	err := xml.Unmarshal(data, entity)
	if err != nil && strings.Contains(err.Error(), "<EntitiesDescriptor>") {
		entities := &saml.EntitiesDescriptor{}
		if err := xml.Unmarshal(data, entities); err != nil {
			return nil, errors.Wrap(err, "unmarshal EntitiesDescriptor")
		}
		for i, e := range entities.EntityDescriptors {
			if len(e.IDPSSODescriptors) > 0 {
				return &entities.EntityDescriptors[i], nil
			}
		}
		return nil, errors.New("no entity with IDPSSODescriptor")
	}
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal EntityDescriptor")
	}
	if len(entity.IDPSSODescriptors) == 0 {
		return nil, errors.New("no IDPSSODescriptor")
	}
	return entity, nil
}

const maxMetadataSize = 1 << 20

// fetchIdPMetadata downloads and parses the IdP metadata.
func fetchIdPMetadata(ctx context.Context, client *http.Client, metadataURL string) (*saml.EntityDescriptor, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "new request")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "fetch metadata")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("fetch metadata: unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataSize))
	if err != nil {
		return nil, errors.Wrap(err, "read metadata")
	}
	return parseIdPMetadata(data)
}
