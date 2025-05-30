package proxy

import (
	"context"
	"net"
	sysnet "net"
	"net/url"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/k3s-io/k3s/pkg/agent/loadbalancer"
	pkgerrors "github.com/pkg/errors"
)

type Proxy interface {
	Update(addresses []string)
	SetAPIServerPort(port int, isIPv6 bool) error
	SetSupervisorDefault(address string)
	IsSupervisorLBEnabled() bool
	SupervisorURL() string
	SupervisorAddresses() []string
	APIServerURL() string
	IsAPIServerLBEnabled() bool
	SetHealthCheck(address string, healthCheck loadbalancer.HealthCheckFunc)
}

// NewSupervisorProxy sets up a new proxy for retrieving supervisor and apiserver addresses.  If
// lbEnabled is true, a load-balancer is started on the requested port to connect to the supervisor
// address, and the address of this local load-balancer is returned instead of the actual supervisor
// and apiserver addresses.
// NOTE: This is a proxy in the API sense - it returns either actual server URLs, or the URL of the
// local load-balancer. It is not actually responsible for proxying requests at the network level;
// this is handled by the load-balancers that the proxy optionally steers connections towards.
func NewSupervisorProxy(ctx context.Context, lbEnabled bool, dataDir, supervisorURL string, lbServerPort int, isIPv6 bool) (Proxy, error) {
	p := proxy{
		lbEnabled:            lbEnabled,
		dataDir:              dataDir,
		initialSupervisorURL: supervisorURL,
		supervisorURL:        supervisorURL,
		apiServerURL:         supervisorURL,
		lbServerPort:         lbServerPort,
		context:              ctx,
	}

	if lbEnabled {
		if err := loadbalancer.SetHTTPProxy(supervisorURL); err != nil {
			return nil, err
		}
		lb, err := loadbalancer.New(ctx, dataDir, loadbalancer.SupervisorServiceName, supervisorURL, p.lbServerPort, isIPv6)
		if err != nil {
			return nil, err
		}
		p.supervisorLB = lb
		p.supervisorURL = lb.LocalURL()
		p.apiServerURL = p.supervisorURL
	}

	u, err := url.Parse(p.initialSupervisorURL)
	if err != nil {
		return nil, pkgerrors.WithMessagef(err, "failed to parse %s", p.initialSupervisorURL)
	}
	p.fallbackSupervisorAddress = u.Host
	p.supervisorPort = u.Port()

	logrus.Debugf("Supervisor proxy using supervisor=%s apiserver=%s lb=%v", p.supervisorURL, p.apiServerURL, p.lbEnabled)
	return &p, nil
}

type proxy struct {
	dataDir          string
	lbEnabled        bool
	lbServerPort     int
	apiServerEnabled bool

	apiServerURL              string
	apiServerPort             string
	supervisorURL             string
	supervisorPort            string
	initialSupervisorURL      string
	fallbackSupervisorAddress string
	supervisorAddresses       []string

	apiServerLB  *loadbalancer.LoadBalancer
	supervisorLB *loadbalancer.LoadBalancer
	context      context.Context
}

func (p *proxy) Update(addresses []string) {
	apiServerAddresses := addresses
	supervisorAddresses := addresses

	if p.apiServerEnabled {
		supervisorAddresses = p.setSupervisorPort(supervisorAddresses)
	}
	if p.apiServerLB != nil {
		p.apiServerLB.Update(apiServerAddresses)
	}
	if p.supervisorLB != nil {
		p.supervisorLB.Update(supervisorAddresses)
	}
	p.supervisorAddresses = supervisorAddresses
}

func (p *proxy) SetHealthCheck(address string, healthCheck loadbalancer.HealthCheckFunc) {
	if p.supervisorLB != nil {
		p.supervisorLB.SetHealthCheck(address, healthCheck)
	}

	if p.apiServerLB != nil {
		host, _, _ := net.SplitHostPort(address)
		address = net.JoinHostPort(host, p.apiServerPort)
		p.apiServerLB.SetHealthCheck(address, healthCheck)
	}
}

func (p *proxy) setSupervisorPort(addresses []string) []string {
	var newAddresses []string
	for _, address := range addresses {
		h, _, err := sysnet.SplitHostPort(address)
		if err != nil {
			logrus.Errorf("Failed to parse address %s, dropping: %v", address, err)
			continue
		}
		newAddresses = append(newAddresses, sysnet.JoinHostPort(h, p.supervisorPort))
	}
	return newAddresses
}

// SetAPIServerPort configures the proxy to return a different set of addresses for the apiserver,
// for use in cases where the apiserver is not running on the same port as the supervisor. If
// load-balancing is enabled, another load-balancer is started on a port one below the supervisor
// load-balancer, and the address of this load-balancer is returned instead of the actual apiserver
// addresses.
func (p *proxy) SetAPIServerPort(port int, isIPv6 bool) error {
	if p.apiServerEnabled {
		logrus.Debugf("Supervisor proxy apiserver port already set")
		return nil
	}

	u, err := url.Parse(p.initialSupervisorURL)
	if err != nil {
		return pkgerrors.WithMessagef(err, "failed to parse server URL %s", p.initialSupervisorURL)
	}
	p.apiServerPort = strconv.Itoa(port)
	u.Host = sysnet.JoinHostPort(u.Hostname(), p.apiServerPort)

	if p.lbEnabled && p.apiServerLB == nil {
		lbServerPort := p.lbServerPort
		if lbServerPort != 0 {
			lbServerPort = lbServerPort - 1
		}
		lb, err := loadbalancer.New(p.context, p.dataDir, loadbalancer.APIServerServiceName, u.String(), lbServerPort, isIPv6)
		if err != nil {
			return err
		}
		p.apiServerLB = lb
		p.apiServerURL = lb.LocalURL()
	} else {
		p.apiServerURL = u.String()
	}

	logrus.Debugf("Supervisor proxy apiserver port changed; apiserver=%s lb=%v", p.apiServerURL, p.lbEnabled)
	p.apiServerEnabled = true
	return nil
}

// SetSupervisorDefault updates the default (fallback) address for the connection to the
// supervisor. This is most useful on k3s nodes without apiservers, where the local
// supervisor must be used to bootstrap the agent config, but then switched over to
// another node running an apiserver once one is available.
func (p *proxy) SetSupervisorDefault(address string) {
	host, port, err := sysnet.SplitHostPort(address)
	if err != nil {
		logrus.Errorf("Failed to parse address %s, dropping: %v", address, err)
		return
	}
	if p.apiServerEnabled {
		port = p.supervisorPort
		address = sysnet.JoinHostPort(host, port)
	}
	p.fallbackSupervisorAddress = address
	if p.supervisorLB == nil {
		p.supervisorURL = "https://" + address
	} else {
		p.supervisorLB.SetDefault(address)
	}
}

func (p *proxy) IsSupervisorLBEnabled() bool {
	return p.supervisorLB != nil
}

func (p *proxy) SupervisorURL() string {
	return p.supervisorURL
}

func (p *proxy) SupervisorAddresses() []string {
	if len(p.supervisorAddresses) > 0 {
		return p.supervisorAddresses
	}
	return []string{p.fallbackSupervisorAddress}
}

func (p *proxy) APIServerURL() string {
	return p.apiServerURL
}

func (p *proxy) IsAPIServerLBEnabled() bool {
	return p.apiServerLB != nil
}
