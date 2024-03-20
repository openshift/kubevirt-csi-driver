package kubevirt

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
	fakecdi "kubevirt.io/client-go/generated/containerized-data-importer/clientset/versioned/fake"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	storageClassName         = "test-storage-class"
	testVolumeName           = "test-volume"
	testVolumeNameNotAllowed = "test-volume-not-allowed"
	validDataVolume          = "pvc-valid-data-volume"
	nolabelDataVolume        = "nolabel-data-volume"
	testClaimName            = "pvc-valid-data-volume"
	testClaimName2           = "pvc-valid-data-volume2"
	testClaimName3           = "pvc-valid-data-volume3"
	testNamespace            = "test-namespace"
	unboundTestClaimName     = "unbound-test-claim"
)

var _ = Describe("Client", func() {
	var (
		c *client
	)

	Context("volumes", func() {
		BeforeEach(func() {
			// Setup code before each test
			c = NewFakeClient()
			c = NewFakeCdiClient(c, createValidDataVolume(), createNoLabelDataVolume(), createWrongPrefixDataVolume())
		})

		DescribeTable("GetDataVolume should return the right thing", func(volumeName string, expectedErr error) {
			_, err := c.GetDataVolume(testNamespace, volumeName)
			if expectedErr != nil {
				Expect(err).To(Equal(expectedErr))
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		},
			Entry("when the data volume exists", validDataVolume, nil),
			Entry("when the data volume exists, but no labels", nolabelDataVolume, ErrInvalidVolume),
			Entry("when the data volume exists, but no labels", testVolumeName, ErrInvalidVolume),
		)

		It("should return not exists if the data volume does not exist", func() {
			_, err := c.GetDataVolume(testNamespace, "notexist")
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("DeleteDataVolume should not delete volumes if the right prefix doesn't exist", func() {
			err := c.DeleteDataVolume(testNamespace, testVolumeName)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(ErrInvalidVolume))
		})

		It("DeleteDataVolume return nil if volume doesn't exist", func() {
			err := c.DeleteDataVolume(testNamespace, "notexist")
			Expect(err).ToNot(HaveOccurred())
		})

		It("DeleteDataVolume should delete volumes if valid", func() {
			err := c.DeleteDataVolume(testNamespace, validDataVolume)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should create a volume if a valid volume is passed", func() {
			dataVolume := createValidDataVolume()
			dataVolume.Name = "pvc-test2"
			_, err := c.CreateDataVolume(testNamespace, dataVolume)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should not create a volume if an invalid volume name is passed", func() {
			dataVolume := createValidDataVolume()
			dataVolume.Name = "test"
			_, err := c.CreateDataVolume(testNamespace, dataVolume)
			Expect(err).To(Equal(ErrInvalidVolume))
		})
	})
})

func NewFakeCdiClient(c *client, objects ...runtime.Object) *client {
	fakeCdiClient := fakecdi.NewSimpleClientset(objects...)
	c.cdiClient = fakeCdiClient
	return c
}

func NewFakeClient() *client {
	testVolume := createPersistentVolume(testVolumeName, storageClassName)
	testVolumeNotAllowed := createPersistentVolume(testVolumeNameNotAllowed, "not-allowed-storage-class")
	testClaim := createPersistentVolumeClaim(testClaimName, testVolumeName, storageClassName)
	testClaim2 := createPersistentVolumeClaim(testClaimName2, "testVolumeName2", storageClassName)
	testClaim3 := createPersistentVolumeClaim(testClaimName3, testVolumeNameNotAllowed, "not-allowed-storage-class")
	unboundClaim := &k8sv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      unboundTestClaimName,
			Namespace: testNamespace,
		},
		Spec: k8sv1.PersistentVolumeClaimSpec{
			StorageClassName: pointer.String(storageClassName),
		},
	}
	fakeK8sClient := k8sfake.NewSimpleClientset(testVolume, testVolumeNotAllowed, testClaim, testClaim2, testClaim3, unboundClaim)

	result := &client{
		kubernetesClient: fakeK8sClient,
		infraLabelMap:    map[string]string{"test": "test"},
		volumePrefix:     "pvc-",
	}
	return result
}

func createPersistentVolume(name, storageClassName string) *k8sv1.PersistentVolume {
	return &k8sv1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: k8sv1.PersistentVolumeSpec{
			StorageClassName: storageClassName,
		},
	}
}

func createPersistentVolumeClaim(name, volumeName, storageClassName string) *k8sv1.PersistentVolumeClaim {
	return &k8sv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    map[string]string{"test": "test"},
		},
		Spec: k8sv1.PersistentVolumeClaimSpec{
			StorageClassName: pointer.String(storageClassName),
			VolumeName:       volumeName,
		},
	}
}

func createDataVolume(name string, labels map[string]string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    labels,
		},
		Spec: cdiv1.DataVolumeSpec{},
	}
}

func createValidDataVolume() *cdiv1.DataVolume {
	return createDataVolume(validDataVolume, map[string]string{"test": "test"})
}

func createNoLabelDataVolume() *cdiv1.DataVolume {
	return createDataVolume(nolabelDataVolume, nil)
}

func createWrongPrefixDataVolume() *cdiv1.DataVolume {
	return createDataVolume(testVolumeName, map[string]string{"test": "test"})
}
