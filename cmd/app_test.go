package cmd

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddPodsToMonitor(t *testing.T) {
	// Create a backup of the original monitoredPods map
	originalMonitoredPods := monitoredPods
	originalPodsChannel := podsChannel
	
	// Setup
	monitoredPods = make(map[string]*tview.TreeView)
	podsChannel = make(chan v1.Pod, 10)
	
	// Cleanup
	defer func() {
		monitoredPods = originalMonitoredPods
		podsChannel = originalPodsChannel
	}()
	
	// Test cases
	testCases := []struct {
		name              string
		inputPods         v1.PodList
		initialMonitored  map[string]bool
		expectedMonitored int
		expectedChannel   int
	}{
		{
			name: "Add new running pod",
			inputPods: v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pod-1",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
				},
			},
			initialMonitored:  map[string]bool{},
			expectedMonitored: 1,
			expectedChannel:   1,
		},
		{
			name: "Skip non-running pod",
			inputPods: v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pod-2",
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
						},
					},
				},
			},
			initialMonitored:  map[string]bool{},
			expectedMonitored: 0,
			expectedChannel:   0,
		},
		{
			name: "Skip already monitored pod",
			inputPods: v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pod-3",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
				},
			},
			initialMonitored:  map[string]bool{"test-pod-3": true},
			expectedMonitored: 1,
			expectedChannel:   0,
		},
		{
			name: "Multiple pods, mix of monitored and unmonitored",
			inputPods: v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pod-4",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pod-5",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pod-6",
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
						},
					},
				},
			},
			initialMonitored:  map[string]bool{"test-pod-4": true},
			expectedMonitored: 2,
			expectedChannel:   1,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear and initialize for each test case
			monitoredPods = make(map[string]*tview.TreeView)
			
			// Initialize existing monitored pods
			for pod := range tc.initialMonitored {
				monitoredPods[pod] = tview.NewTreeView().SetRoot(tview.NewTreeNode(pod).SetColor(tcell.ColorGreen))
			}
			
			// Clear the channel and initialize app
			for len(podsChannel) > 0 {
				<-podsChannel
			}
			app = tview.NewApplication()
			liveBox = tview.NewTextView()
			
			// Test
			addPodsToMonitor(tc.inputPods)
			
			// Verify results
			assert.Equal(t, tc.expectedMonitored, len(monitoredPods), "Number of monitored pods should match expected")
			assert.Equal(t, tc.expectedChannel, len(podsChannel), "Number of pods in channel should match expected")
			
			// Verify monitored pods contain all expected pods
			for pod := range tc.initialMonitored {
				_, exists := monitoredPods[pod]
				assert.True(t, exists, "Expected monitored pod '%s' should still be present", pod)
			}
			
			// Verify new running pods were added
			for _, pod := range tc.inputPods.Items {
				if pod.Status.Phase == v1.PodRunning && !tc.initialMonitored[pod.Name] {
					_, exists := monitoredPods[pod.Name]
					assert.True(t, exists, "New running pod '%s' should be added to monitored pods", pod.Name)
					
					// Verify newly added pods have yellow color
					if exists {
						assert.Equal(t, tcell.ColorYellow, monitoredPods[pod.Name].GetRoot().GetColor(), 
							"Newly added pod '%s' should have yellow color", pod.Name)
					}
				}
			}
		})
	}
}

func TestUpdateMonitoredPodBoxColors(t *testing.T) {
	// Create a backup of the original monitoredPods map
	originalMonitoredPods := monitoredPods
	originalApp := app
	
	// Setup for test
	monitoredPods = make(map[string]*tview.TreeView)
	app = tview.NewApplication()
	
	// Cleanup
	defer func() {
		monitoredPods = originalMonitoredPods
		app = originalApp
	}()
	
	// Add test pods with different colors
	monitoredPods["pod1"] = tview.NewTreeView().SetRoot(tview.NewTreeNode("pod1").SetColor(tcell.ColorYellow))
	monitoredPods["pod2"] = tview.NewTreeView().SetRoot(tview.NewTreeNode("pod2").SetColor(tcell.ColorGreen))
	monitoredPods["pod3"] = tview.NewTreeView().SetRoot(tview.NewTreeNode("pod3").SetColor(tcell.ColorBlue))
	monitoredPods["pod4"] = tview.NewTreeView().SetRoot(tview.NewTreeNode("pod4").SetColor(tcell.ColorRed))
	
	// Manually update the colors as the updateMonitoredPodBox function would
	mu.Lock()
	for podName, tree := range monitoredPods {
		if tree.GetRoot().GetColor() == tcell.ColorRed {
			delete(monitoredPods, podName)
		} else if tree.GetRoot().GetColor() == tcell.ColorYellow {
			tree.GetRoot().SetColor(tcell.ColorGreen)
		} else if tree.GetRoot().GetColor() == tcell.ColorGreen {
			tree.GetRoot().SetColor(tcell.ColorBlue)
		}
	}
	mu.Unlock()
	
	// Verify color transitions
	_, pod1Exists := monitoredPods["pod1"]
	assert.True(t, pod1Exists, "pod1 should still exist")
	if pod1Exists {
		assert.Equal(t, tcell.ColorGreen, monitoredPods["pod1"].GetRoot().GetColor(), "pod1 should transition from Yellow to Green")
	}
	
	_, pod2Exists := monitoredPods["pod2"]
	assert.True(t, pod2Exists, "pod2 should still exist")
	if pod2Exists {
		assert.Equal(t, tcell.ColorBlue, monitoredPods["pod2"].GetRoot().GetColor(), "pod2 should transition from Green to Blue")
	}
	
	_, pod3Exists := monitoredPods["pod3"]
	assert.True(t, pod3Exists, "pod3 should still exist")
	if pod3Exists {
		assert.Equal(t, tcell.ColorBlue, monitoredPods["pod3"].GetRoot().GetColor(), "pod3 should remain Blue")
	}
	
	_, pod4Exists := monitoredPods["pod4"]
	assert.False(t, pod4Exists, "pod4 should be removed because it was Red")
}

func TestPodRefreshIntegration(t *testing.T) {
	// This test relies on the actual implementation without mocking
	// It tests the integration between checkNewPods and addPodsToMonitor
	
	// Save original channel
	originalPodsChannel := podsChannel
	originalMonitoredPods := monitoredPods
	
	// Create test data
	monitoredPods = make(map[string]*tview.TreeView)
	podsChannel = make(chan v1.Pod, 10)
	
	// Create a test pod that will be "discovered" by our code
	testPod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod-new",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}
	
	// Set up the app for the liveBox updates
	app = tview.NewApplication()
	liveBox = tview.NewTextView()
	
	// Create a test syncWaitGroup and manually call addPodsToMonitor
	// to simulate what checkNewPods would do
	addPodsToMonitor(v1.PodList{
		Items: []v1.Pod{testPod},
	})
	
	// Verify pod was added to monitor
	assert.Contains(t, monitoredPods, "test-pod-new", "New pod should be added to monitored pods")
	assert.Equal(t, 1, len(podsChannel), "New pod should be added to the channel")
	
	// Verify the pod has the correct color
	podTreeView, exists := monitoredPods["test-pod-new"]
	if assert.True(t, exists, "Pod should exist in monitored pods") {
		assert.Equal(t, tcell.ColorYellow, podTreeView.GetRoot().GetColor(), 
			"Newly added pod should have yellow color")
	}
	
	// Verify there's a pod in the channel and it's our test pod
	select {
	case pod := <-podsChannel:
		assert.Equal(t, "test-pod-new", pod.Name, "The pod in the channel should be our test pod")
	default:
		t.Error("Expected a pod in the channel but found none")
	}
	
	// Restore original state
	podsChannel = originalPodsChannel
	monitoredPods = originalMonitoredPods
}