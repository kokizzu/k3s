package secretsencryption

import (
	"flag"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var hardened = flag.Bool("hardened", false, "true or false")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 or nil for latest commit from master

func Test_E2ESecretsEncryptionOld(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Secrets Encryption Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Secrets Encryption Rotation", Ordered, func() {
	Context("Secrets Keys are rotated:", func() {
		It("Starts up with no issues", func() {
			var err error
			if *local {
				tc, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, 0)
			} else {
				tc, err = e2e.CreateCluster(*nodeOS, *serverCount, 0)
			}
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			tc.Hardened = *hardened
			By("CLUSTER CONFIG")
			By("OS: " + *nodeOS)
			By(tc.Status())

		})

		It("Checks node and pod status", func() {
			By("Fetching Nodes status")
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, e2e.VagrantSlice(tc.AllNodes()))
			}, "620s", "5s").Should(Succeed())

			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "620s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("Deploys several secrets", func() {
			_, err := tc.DeployWorkload("secrets.yaml")
			Expect(err).NotTo(HaveOccurred(), "Secrets not deployed")
		})

		It("Verifies encryption start stage", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				res, err := node.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
				Expect(res).Should(ContainSubstring("Current Rotation Stage: start"))
				Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
			}
		})

		It("Prepares for Secrets-Encryption Rotation", func() {
			cmd := "k3s secrets-encrypt prepare"
			res, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
			for i, node := range tc.Servers {
				cmd := "k3s secrets-encrypt status"
				res, err := node.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred(), res)
				Expect(res).Should(ContainSubstring("Server Encryption Hashes: hash does not match"))
				if i == 0 {
					Expect(res).Should(ContainSubstring("Current Rotation Stage: prepare"))
				} else {
					Expect(res).Should(ContainSubstring("Current Rotation Stage: start"))
				}
			}
		})

		It("Restarts K3s servers", func() {
			Expect(e2e.RestartCluster(tc.Servers)).To(Succeed(), e2e.GetVagrantLog(nil))
		})

		It("Checks node and pod status", func() {
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, e2e.VagrantSlice(tc.AllNodes()))
			}, "360s", "5s").Should(Succeed())

			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "360s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("Verifies encryption prepare stage", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					g.Expect(res).Should(ContainSubstring("Current Rotation Stage: prepare"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Rotates the Secrets-Encryption Keys", func() {
			cmd := "k3s secrets-encrypt rotate"
			res, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
			for i, node := range tc.Servers {
				Eventually(func(g Gomega) {
					cmd := "k3s secrets-encrypt status"
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred(), res)
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: hash does not match"))
					if i == 0 {
						g.Expect(res).Should(ContainSubstring("Current Rotation Stage: rotate"))
					} else {
						g.Expect(res).Should(ContainSubstring("Current Rotation Stage: prepare"))
					}
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Restarts K3s servers", func() {
			Expect(e2e.RestartCluster(tc.Servers)).To(Succeed(), e2e.GetVagrantLog(nil))
		})

		It("Verifies encryption rotate stage", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					g.Expect(res).Should(ContainSubstring("Current Rotation Stage: rotate"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Reencrypts the Secrets-Encryption Keys", func() {
			cmd := "k3s secrets-encrypt reencrypt"
			res, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)

			cmd = "k3s secrets-encrypt status"
			Eventually(func() (string, error) {
				return tc.Servers[0].RunCmdOnNode(cmd)
			}, "240s", "10s").Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))

			for _, node := range tc.Servers[1:] {
				res, err := node.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred(), res)
				Expect(res).Should(ContainSubstring("Server Encryption Hashes: hash does not match"))
				Expect(res).Should(ContainSubstring("Current Rotation Stage: rotate"))
			}
		})

		It("Restarts K3s Servers", func() {
			Expect(e2e.RestartCluster(tc.Servers)).To(Succeed(), e2e.GetVagrantLog(nil))
		})

		It("Verifies Encryption Reencrypt Stage", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					g.Expect(res).Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "420s", "5s").Should(Succeed())
			}
		})
	})

	Context("Disabling Secrets-Encryption", func() {
		It("Disables encryption", func() {
			cmd := "k3s secrets-encrypt disable"
			res, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)

			cmd = "k3s secrets-encrypt reencrypt -f --skip"
			res, err = tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)

			cmd = "k3s secrets-encrypt status"
			Eventually(func() (string, error) {
				return tc.Servers[0].RunCmdOnNode(cmd)
			}, "240s", "10s").Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))

			for i, node := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred(), res)
					if i == 0 {
						g.Expect(res).Should(ContainSubstring("Encryption Status: Disabled"))
					} else {
						g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					}
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Restarts K3s servers", func() {
			Expect(e2e.RestartCluster(tc.Servers)).To(Succeed())
		})

		It("Verifies encryption disabled on all nodes", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				Eventually(func(g Gomega) {
					g.Expect(node.RunCmdOnNode(cmd)).Should(ContainSubstring("Encryption Status: Disabled"))
				}, "420s", "2s").Should(Succeed())
			}
		})

	})

	Context("Enabling Secrets-Encryption", func() {
		It("Enables encryption", func() {
			cmd := "k3s secrets-encrypt enable"
			res, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)

			cmd = "k3s secrets-encrypt reencrypt -f --skip"
			res, err = tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)

			cmd = "k3s secrets-encrypt status"
			Eventually(func() (string, error) {
				return tc.Servers[0].RunCmdOnNode(cmd)
			}, "180s", "5s").Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))
		})

		It("Restarts K3s servers", func() {
			Expect(e2e.RestartCluster(tc.Servers)).To(Succeed())
		})

		It("Verifies encryption enabled on all nodes", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				Eventually(func(g Gomega) {
					g.Expect(node.RunCmdOnNode(cmd)).Should(ContainSubstring("Encryption Status: Enabled"))
				}, "420s", "2s").Should(Succeed())
			}

		})
	})

})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if !failed {
		Expect(e2e.GetCoverageReport(tc.Servers)).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
