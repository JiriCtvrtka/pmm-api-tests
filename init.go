package pmmapitests

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/percona/pmm/api/alertmanager/amclient"
	inventoryClient "github.com/percona/pmm/api/inventorypb/json/client"
	managementClient "github.com/percona/pmm/api/managementpb/json/client"
	serverClient "github.com/percona/pmm/api/serverpb/json/client"
	"github.com/percona/pmm/utils/tlsconfig"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

//nolint:gochecknoglobals
var (
	// Context is canceled on SIGTERM or SIGINT. Tests should cleanup and exit.
	Context context.Context

	// BaseURL contains PMM Server base URL like https://admin:admin@127.0.0.1:8443/.
	BaseURL *url.URL

	// Hostname contains local hostname that is used for generating test data.
	Hostname string

	// True if -debug or -trace flag is passed.
	Debug bool

	// RunUpdateTest is true if PMM Server update should be tested.
	RunUpdateTest bool

	// RunSTTTests is true if STT tests should be run.
	RunSTTTests bool
)

// ErrFromNginx is an error type for nginx HTML response.
type ErrFromNginx string

// Error implements error interface.
func (e *ErrFromNginx) Error() string {
	return "response from nginx: " + string(*e)
}

// GoString implements fmt.GoStringer interface.
func (e *ErrFromNginx) GoString() string {
	return fmt.Sprintf("ErrFromNginx(%q)", string(*e))
}

// Transport returns configured Swagger transport for given URL.
func Transport(baseURL *url.URL, insecureTLS bool) *httptransport.Runtime {
	transport := httptransport.New(baseURL.Host, baseURL.Path, []string{baseURL.Scheme})
	if u := baseURL.User; u != nil {
		password, _ := u.Password()
		transport.DefaultAuthentication = httptransport.BasicAuth(u.Username(), password)
	}
	transport.SetLogger(logrus.WithField("component", "client"))
	transport.SetDebug(logrus.GetLevel() >= logrus.DebugLevel)
	transport.Context = context.Background() // not Context - do not cancel the whole transport

	// set error handlers for nginx responses if pmm-managed is down
	errorConsumer := runtime.ConsumerFunc(func(reader io.Reader, data interface{}) error {
		b, _ := ioutil.ReadAll(reader)
		err := ErrFromNginx(string(b))
		return &err
	})
	transport.Consumers = map[string]runtime.Consumer{
		runtime.JSONMime:    runtime.JSONConsumer(),
		runtime.HTMLMime:    errorConsumer,
		runtime.TextMime:    errorConsumer,
		runtime.DefaultMime: errorConsumer,
	}

	// disable HTTP/2, set TLS config
	httpTransport := transport.Transport.(*http.Transport)
	httpTransport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	if baseURL.Scheme == "https" {
		httpTransport.TLSClientConfig = tlsconfig.Get()
		httpTransport.TLSClientConfig.ServerName = baseURL.Hostname()
		httpTransport.TLSClientConfig.InsecureSkipVerify = insecureTLS
	}

	return transport
}

//nolint:gochecknoinits
func init() {
	rand.Seed(time.Now().UnixNano())

	debugF := flag.Bool("pmm.debug", false, "Enable debug output [PMM_DEBUG].")
	traceF := flag.Bool("pmm.trace", false, "Enable trace output [PMM_TRACE].")
	serverURLF := flag.String("pmm.server-url", "https://admin:admin@127.0.0.1:443/", "PMM Server URL [PMM_SERVER_URL].")
	serverInsecureTLSF := flag.Bool("pmm.server-insecure-tls", false, "Skip PMM Server TLS certificate validation [PMM_SERVER_INSECURE_TLS].")
	runUpdateTestF := flag.Bool("pmm.run-update-test", false, "Run PMM Server update test [PMM_RUN_UPDATE_TEST].")

	// FIXME we should rethink it once https://jira.percona.com/browse/PMM-5106 is implemented
	runSTTTestsF := flag.Bool("pmm.run-stt-tests", false, "Run STT tests that require connected clients [PMM_RUN_STT_TESTS].")

	testing.Init()
	flag.Parse()

	for envVar, f := range map[string]*flag.Flag{
		"PMM_DEBUG":               flag.Lookup("pmm.debug"),
		"PMM_TRACE":               flag.Lookup("pmm.trace"),
		"PMM_SERVER_URL":          flag.Lookup("pmm.server-url"),
		"PMM_SERVER_INSECURE_TLS": flag.Lookup("pmm.server-insecure-tls"),
		"PMM_RUN_UPDATE_TEST":     flag.Lookup("pmm.run-update-test"),
		"PMM_RUN_STT_TESTS":       flag.Lookup("pmm.run-stt-tests"),
	} {
		env, ok := os.LookupEnv(envVar)
		if ok {
			err := f.Value.Set(env)
			if err != nil {
				logrus.Fatalf("Invalid ENV variable %s: %s", envVar, env)
			}
		}
	}

	if *debugF {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if *traceF {
		logrus.SetLevel(logrus.TraceLevel)
		logrus.SetReportCaller(true)
	}
	Debug = *debugF || *traceF
	RunUpdateTest = *runUpdateTestF
	RunSTTTests = *runSTTTestsF

	var cancel context.CancelFunc
	Context, cancel = context.WithCancel(context.Background())

	// handle termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-signals
		signal.Stop(signals)
		logrus.Warnf("Got %s, shutting down...", unix.SignalName(s.(syscall.Signal)))
		cancel()
	}()

	var err error
	BaseURL, err = url.Parse(*serverURLF)
	if err != nil {
		logrus.Fatalf("Failed to parse PMM Server URL: %s.", err)
	}
	if BaseURL.Host == "" || BaseURL.Scheme == "" {
		logrus.Fatalf("Invalid PMM Server URL: %s", BaseURL.String())
	}
	if BaseURL.Path == "" {
		BaseURL.Path = "/"
	}
	logrus.Debugf("PMM Server URL: %s.", BaseURL)

	Hostname, err = os.Hostname()
	if err != nil {
		logrus.Fatalf("Failed to detect hostname: %s", err)
	}

	transport := Transport(BaseURL, *serverInsecureTLSF)
	alertmanagerTransport := Transport(BaseURL, *serverInsecureTLSF)
	alertmanagerTransport.BasePath = "/alertmanager/api/v2"
	transport.Consumers["application/zip"] = runtime.ByteStreamConsumer()
	inventoryClient.Default = inventoryClient.New(transport, nil)
	managementClient.Default = managementClient.New(transport, nil)
	serverClient.Default = serverClient.New(transport, nil)
	amclient.Default = amclient.New(alertmanagerTransport, nil)

	// do not run tests if server is not available
	_, err = serverClient.Default.Server.Readiness(nil)
	if err != nil {
		panic(err)
	}
}

// check interfaces
var (
	_ error          = (*ErrFromNginx)(nil)
	_ fmt.GoStringer = (*ErrFromNginx)(nil)
)
