// Copyright 2023 The ClusterLink Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/clusterlink-net/clusterlink/pkg/controlplane/api"
	"github.com/clusterlink-net/clusterlink/pkg/controlplane/eventmanager"
)

const (
	// the number of seconds a JWT access token is valid before it expires.
	jwtExpirySeconds      = 5
	jwtSignatureAlgorithm = jwa.RS256
)

// EgressAuthorizationRequest (from local dataplane) represents a request for accessing an imported service.
type EgressAuthorizationRequest struct {
	// Import is the name of the requested imported service.
	Import string
	// IP address of the client connecting to the service.
	IP string
}

// EgressAuthorizationResponse (to local dataplane) represents a response for an EgressAuthorizationRequest.
type EgressAuthorizationResponse struct {
	// ServiceExists is true if the requested service exists.
	ServiceExists bool
	// Allowed is true if the request is allowed.
	Allowed bool
	// RemotePeerCluster is the cluster name of the remote peer where the connection should be routed to.
	RemotePeerCluster string
	// AccessToken is a token that allows accessing the requested service.
	AccessToken string
}

// IngressAuthorizationRequest (to remote peer controlplane) represents a request for accessing an exported service.
type IngressAuthorizationRequest struct {
	// Service is the name of the requested exported service.
	Service string
}

// IngressAuthorizationResponse (from remote peer controlplane)
// represents a response for an IngressAuthorizationRequest.
type IngressAuthorizationResponse struct {
	// ServiceExists is true if the requested service exists.
	ServiceExists bool
	// Allowed is true if the request is allowed.
	Allowed bool
	// AccessToken is a token that allows accessing the requested service.
	AccessToken string
}

// AuthorizeEgress authorizes a request for accessing an imported service.
func (cp *Instance) AuthorizeEgress(req *EgressAuthorizationRequest) (*EgressAuthorizationResponse, error) {
	cp.logger.Infof("Received egress authorization request: %v.", req)

	if imp := cp.GetImport(req.Import); imp == nil {
		return nil, fmt.Errorf("import '%s' not found", req.Import)
	}

	bindings := cp.GetBindings(req.Import)
	if len(bindings) == 0 {
		return nil, fmt.Errorf("no bindings found for import '%s'", req.Import)
	}

	connReq := eventmanager.ConnectionRequestAttr{DstService: req.Import, Direction: eventmanager.Outgoing}
	srcLabels := cp.platform.GetLabelsFromIP(req.IP)
	if src, ok := srcLabels["app"]; ok { // TODO: Add support for labels other than just the "app" key.
		cp.logger.Infof("Received egress authorization srcLabels[app]: %v.", srcLabels["app"])
		connReq.SrcService = src
	}

	authResp, err := cp.policyDecider.AuthorizeAndRouteConnection(&connReq)
	if err != nil {
		return nil, err
	}

	if authResp.Action != eventmanager.Allow {
		return &EgressAuthorizationResponse{Allowed: false}, nil
	}

	target := authResp.TargetPeer
	peer := cp.GetPeer(target)
	if peer == nil {
		return nil, fmt.Errorf("peer '%s' does not exist", target)
	}

	cp.peerLock.RLock()
	client, ok := cp.peerClient[peer.Name]
	cp.peerLock.RUnlock()

	if !ok {
		return nil, fmt.Errorf("missing client for peer: %s", peer.Name)
	}

	serverResp, err := client.Authorize(&api.AuthorizationRequest{Service: req.Import})
	if err != nil {
		return nil, fmt.Errorf("unable to get access token from peer: %w", err)
	}

	resp := &EgressAuthorizationResponse{
		ServiceExists: serverResp.ServiceExists,
		Allowed:       serverResp.Allowed,
	}

	if serverResp.Allowed {
		resp.RemotePeerCluster = api.RemotePeerClusterName(peer.Name)
		resp.AccessToken = serverResp.AccessToken
	}

	return resp, nil
}

// AuthorizeIngress authorizes a request for accessing an exported service.
func (cp *Instance) AuthorizeIngress(req *IngressAuthorizationRequest, peer string) (*IngressAuthorizationResponse, error) {
	cp.logger.Infof("Received ingress authorization request: %v.", req)

	resp := &IngressAuthorizationResponse{}

	export := cp.GetExport(req.Service)
	if export == nil {
		return resp, nil
	}

	resp.ServiceExists = true

	connReq := eventmanager.ConnectionRequestAttr{
		DstService: req.Service,
		Direction:  eventmanager.Incoming,
		OtherPeer:  peer,
	}
	authResp, err := cp.policyDecider.AuthorizeAndRouteConnection(&connReq)
	if err != nil {
		return nil, err
	}
	if authResp.Action != eventmanager.Allow {
		resp.Allowed = false
		return resp, nil
	}
	resp.Allowed = true

	// create access token
	// TODO: include client name as a claim
	token, err := jwt.NewBuilder().
		Expiration(time.Now().Add(time.Second*jwtExpirySeconds)).
		Claim(api.ExportNameJWTClaim, export.Name).
		Build()
	if err != nil {
		return nil, fmt.Errorf("unable to generate access token: %w", err)
	}

	// sign access token
	signed, err := jwt.Sign(token, jwtSignatureAlgorithm, cp.jwkSignKey)
	if err != nil {
		return nil, fmt.Errorf("unable to sign access token: %w", err)
	}
	resp.AccessToken = string(signed)

	return resp, nil
}

// ParseAuthorizationHeader verifies an access token for an ingress dataplane connection.
// On success, returns the parsed target cluster name.
func (cp *Instance) ParseAuthorizationHeader(token string) (string, error) {
	cp.logger.Debug("Parsing access token.")

	parsedToken, err := jwt.ParseString(
		token, jwt.WithVerify(jwtSignatureAlgorithm, cp.jwkVerifyKey), jwt.WithValidate(true))
	if err != nil {
		return "", err
	}

	// TODO: verify client name

	export, ok := parsedToken.PrivateClaims()[api.ExportNameJWTClaim]
	if !ok {
		return "", fmt.Errorf("token missing '%s' claim", api.ExportNameJWTClaim)
	}

	return api.ExportClusterName(export.(string)), nil
}
