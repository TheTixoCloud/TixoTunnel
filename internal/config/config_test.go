// Configuration safety regressions by ir_spoof. https://t.me/ir_spoof
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testConfig = `[listener]
bind_addr = "127.0.0.1:4000"
[transport]
type = "tcp"
[security]
token = "a-long-example-peer-secret-for-tests-only"
[tuning]
auto_tuning = false
`

func readTest(t *testing.T, s string) (Config, error) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.toml")
	if e := os.WriteFile(p, []byte(s), 0600); e != nil {
		t.Fatal(e)
	}
	return Load(p)
}
func TestConfigurationRejectsUnsafeAndUnknownInputs(t *testing.T) {
	if _, err := readTest(t, testConfig); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"unknown":          testConfig + "made_up_option = true\n",
		"negative_timeout": testConfig + "read_timeout = -1\n",
		"overflow_port":    strings.Replace(testConfig, "4000", "18446744073709551617", 1),
		"autotuning":       strings.Replace(testConfig, "auto_tuning = false", "auto_tuning = true", 1),
		"cipher":           testConfig + "\n[tls]\nnot_a_cert = true\n",
		"source_hostname":  "[dialer]\nremote_addr=\"localhost:4000\"\nlocal_addr=\"example.invalid\"\n[security]\ntoken=\"a-long-example-peer-secret-for-tests-only\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readTest(t, value); err == nil {
				t.Fatal("unsafe configuration was accepted")
			}
		})
	}
}
func TestConfigurationDoesNotLookUpConfiguredHostnames(t *testing.T) {
	c, err := readTest(t, "[dialer]\nremote_addr=\"unresolved-peer.invalid:4000\"\n[security]\ntoken=\"a-long-example-peer-secret-for-tests-only\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if c.Server() {
		t.Fatal("dialer incorrectly classified")
	}
}
func TestTLSListenerRequiresUsableLocalFiles(t *testing.T) {
	_, err := readTest(t, strings.Replace(testConfig, "type = \"tcp\"", "type = \"wss\"", 1))
	if err == nil {
		t.Fatal("missing TLS identity accepted")
	}
}
func TestStrictBoundedJSON(t *testing.T) {
	for _, s := range []string{`{"listener":{"bind_addr":"127.0.0.1:4000"},"security":{"token":"a-long-example-peer-secret-for-tests-only"},"unknown":true}`, strings.Repeat("x", 1024*1024+1)} {
		if _, err := readTest(t, s); err == nil {
			t.Fatal("invalid document accepted")
		}
	}
}
