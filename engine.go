package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	serverAddr = "127.0.0.1:14000"
	authPass   = "my_secure_password"
	delimiter  = "<???DONE???---"
)

type Message map[string]interface{}

type Cube struct {
	Name     string
	Position []float64
}

type CubeLink struct {
	JointName string
	CubeA     string
	CubeB     string
}

var (
	globalCubeList  []string
	cubeListMutex   sync.Mutex
	globalCubeLinks []CubeLink
	linkListMutex   sync.Mutex
)

func sendJSONMessage(conn net.Conn, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, []byte(delimiter)...)
	_, err = conn.Write(data)
	return err
}

func readResponse(conn net.Conn) (string, error) {
	reader := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var builder strings.Builder
	for {
		line, err := reader.ReadString('-')
		if err != nil {
			break
		}
		builder.WriteString(line)
		if strings.Contains(line, delimiter) {
			break
		}
	}
	full := strings.ReplaceAll(builder.String(), delimiter, "")
	return strings.TrimSpace(full), nil
}

func spawnCube(cube Cube, wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("[Spawn] Failed to connect:", err)
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
		fmt.Println("[Spawn] Auth write error:", err)
		return
	}
	_, err = readResponse(conn)
	if err != nil {
		fmt.Println("[Spawn] Failed to read auth response:", err)
		return
	}

	spawn := Message{
		"type":      "spawn_cube",
		"cube_name": cube.Name,
		"position":  cube.Position,
		"rotation":  []float64{0, 0, 0},
		"is_base":   true,
	}
	if err := sendJSONMessage(conn, spawn); err != nil {
		fmt.Println("[Spawn] Failed to spawn cube:", err)
		return
	}

	fullCubeName := cube.Name + "_BASE"
	cubeListMutex.Lock()
	globalCubeList = append(globalCubeList, fullCubeName)
	cubeListMutex.Unlock()
}

func unfreezeAllCubes() {
	var wg sync.WaitGroup
	for _, cube := range globalCubeList {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			conn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Println("[Unfreeze] Failed to connect:", err)
				return
			}
			defer conn.Close()

			if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
				return
			}
			_, _ = readResponse(conn)

			unfreeze := Message{
				"type":      "freeze_cube",
				"cube_name": name,
				"freeze":    false,
			}
			sendJSONMessage(conn, unfreeze)
		}(cube)
	}
	wg.Wait()
}

func despawnAllCubes() {
	var wg sync.WaitGroup
	for _, cube := range globalCubeList {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			conn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Println("[Despawn] Failed to connect:", err)
				return
			}
			defer conn.Close()

			if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
				return
			}
			_, _ = readResponse(conn)

			despawn := Message{
				"type":      "despawn_cube",
				"cube_name": name,
			}
			sendJSONMessage(conn, despawn)
		}(cube)
	}
	wg.Wait()
}

func linkCubes(cubeA, cubeB, jointType, jointName string) {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("[Link] Failed to connect:", err)
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
		fmt.Println("[Link] Auth write error:", err)
		return
	}
	_, _ = readResponse(conn)

	link := Message{
		"type":       "create_joint",
		"cube1":      cubeA,
		"cube2":      cubeB,
		"joint_type": jointType,
		"joint_name": jointName,
	}

	if err := sendJSONMessage(conn, link); err != nil {
		fmt.Println("[Link] Failed to send link command:", err)
		return
	}

	linkListMutex.Lock()
	globalCubeLinks = append(globalCubeLinks, CubeLink{
		JointName: jointName,
		CubeA:     cubeA,
		CubeB:     cubeB,
	})
	linkListMutex.Unlock()

	fmt.Printf("🔗 Linked %s <--> %s with joint '%s' (%s)\n", cubeA, cubeB, jointName, jointType)
}

// setJointParam sends a JSON command to set a specific parameter for a joint.
func setJointParam(conn net.Conn, jointName, paramName string, value float64) {
	// Build the command message.
	cmd := Message{
		"type":       "set_joint_param",
		"joint_name": jointName,
		"param_name": paramName,
		"value":      value,
	}
	// Send the JSON command.
	if err := sendJSONMessage(conn, cmd); err != nil {
		fmt.Printf("[setJointParam] Failed to send command for joint %s: %v\n", jointName, err)
		return
	}
	// Optionally, read the server response.
	resp, err := readResponse(conn)
	if err != nil {
		fmt.Printf("[setJointParam] Error reading response for joint %s: %v\n", jointName, err)
		return
	}
	fmt.Printf("[setJointParam] Joint %s param %s set to %v, response: %s\n", jointName, paramName, value, resp)
}

// stiffenAllJoints opens a TCP connection, authenticates, and then loops over all joints
// (stored in globalCubeLinks) to apply a set of stiffening parameters.
func SingleThreadedstiffenAllJoints() {
	// Open a connection.
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("[stiffenAllJoints] Failed to connect:", err)
		return
	}
	defer conn.Close()

	// Authenticate.
	if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
		fmt.Println("[stiffenAllJoints] Auth write error:", err)
		return
	}
	_, err = readResponse(conn)
	if err != nil {
		fmt.Println("[stiffenAllJoints] Failed to read auth response:", err)
		return
	}

	// Define the parameters to enforce stiffness.
	/*params := map[string]float64{
		"limit_upper":           0.0, // both 0 => no swing
		"limit_lower":           0.0,
		"motor_enable":          1.0,
		"motor_target_velocity": 0.0,
		"motor_max_impulse":     1000.0,
		"limit_softness":        1.0,
		"limit_bias":            0.9,
		"limit_relaxation":      1.0,
	}*/

	params := map[string]float64{
		"limit_upper":           0.0,
		"limit_lower":           0.0,
		"motor_enable":          1.0,
		"motor_target_velocity": 0.0,
		"motor_max_impulse":     1000.0,
	}

	// Loop over each joint stored in globalCubeLinks.
	for _, link := range globalCubeLinks {
		for param, value := range params {
			setJointParam(conn, link.JointName, param, value)
		}
	}
}

func SingleTCPConnectionExamplestiffenAllJoints() {
	// Define the parameters for stiffening.
	params := map[string]float64{
		"limit_upper":           0.0,
		"limit_lower":           0.0,
		"motor_enable":          1.0,
		"motor_target_velocity": 0.0,
		"motor_max_impulse":     1000.0,
	}

	// 1) Open ONE TCP connection for all joints.
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("[stiffenAllJoints] Failed to connect:", err)
		return
	}
	defer conn.Close()

	// Authenticate once.
	if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
		fmt.Println("[stiffenAllJoints] Auth write error:", err)
		return
	}
	if _, err := readResponse(conn); err != nil {
		fmt.Println("[stiffenAllJoints] Auth response error:", err)
		return
	}

	// 2) For each joint in globalCubeLinks...
	for _, link := range globalCubeLinks {
		// 3) For each parameter, send the command via setJointParam.
		for paramName, val := range params {
			setJointParam(conn, link.JointName, paramName, val)

			// (Optional) read server confirmation if your setJointParam
			// doesn't already do that internally.
			// resp, err := readResponse(conn)
			// if err != nil {
			//     fmt.Printf("[stiffenAllJoints] Error reading response for joint %s: %v\n", link.JointName, err)
			// }
		}
	}

	fmt.Println("[stiffenAllJoints] All joints have been stiffened using a single connection.")
}

func stiffenAllJoints() {
	// The parameter set we want for each joint.
	params := map[string]float64{
		"limit_upper":           0.0,
		"limit_lower":           0.0,
		"motor_enable":          1.0,
		"motor_target_velocity": 0.0,
		"motor_max_impulse":     1000.0,
	}

	// We'll spawn one goroutine per joint in globalCubeLinks.
	var wg sync.WaitGroup
	for _, link := range globalCubeLinks {
		wg.Add(1)
		go func(joint CubeLink) {
			defer wg.Done()

			// Open a fresh TCP connection for this joint.
			conn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Printf("[stiffenAllJoints] Failed to connect for joint %s: %v\n", joint.JointName, err)
				return
			}
			defer conn.Close()

			// Authenticate
			if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
				fmt.Printf("[stiffenAllJoints] Auth write error for joint %s: %v\n", joint.JointName, err)
				return
			}
			if _, err := readResponse(conn); err != nil {
				fmt.Printf("[stiffenAllJoints] Auth response error for joint %s: %v\n", joint.JointName, err)
				return
			}

			// For each parameter, set it on this joint.
			for paramName, val := range params {
				setJointParam(conn, joint.JointName, paramName, val)
			}
		}(link)
	}

	// Wait for all joint goroutines to finish.
	wg.Wait()
	fmt.Println("[stiffenAllJoints] All joints have been stiffened.")
}

func main() {
	// Position offset for moving the whole structure
	var offset = []float64{40, -20, -3} // Example: move dog +10 X, +5 Y, -3 Z

	// Cube groups with offset applied manually
	cubeGroups := [][]Cube{
		{
			{Name: "rightear", Position: []float64{0 + offset[0], 130 + offset[1], 0 + offset[2]}},
			{Name: "leftear", Position: []float64{2 + offset[0], 130 + offset[1], 0 + offset[2]}},
		},
		{
			{Name: "head1", Position: []float64{0 + offset[0], 128.9 + offset[1], 0 + offset[2]}},
			{Name: "head2", Position: []float64{1 + offset[0], 128.9 + offset[1], 0 + offset[2]}},
			{Name: "head3", Position: []float64{2 + offset[0], 128.9 + offset[1], 0 + offset[2]}},
			{Name: "head4", Position: []float64{0 + offset[0], 128.9 + offset[1], 1 + offset[2]}},
			{Name: "head5", Position: []float64{1 + offset[0], 128.9 + offset[1], 1 + offset[2]}},
			{Name: "head6", Position: []float64{2 + offset[0], 128.9 + offset[1], 1 + offset[2]}},
			{Name: "head7", Position: []float64{0 + offset[0], 127.9 + offset[1], 0 + offset[2]}},
			{Name: "head8", Position: []float64{1 + offset[0], 127.9 + offset[1], 0 + offset[2]}},
			{Name: "head9", Position: []float64{2 + offset[0], 127.9 + offset[1], 0 + offset[2]}},
			{Name: "head10", Position: []float64{0 + offset[0], 127.9 + offset[1], 1 + offset[2]}},
			{Name: "head11", Position: []float64{1 + offset[0], 127.9 + offset[1], 1 + offset[2]}},
			{Name: "head12", Position: []float64{2 + offset[0], 127.9 + offset[1], 1 + offset[2]}},
		},
		{
			{Name: "leftmouth", Position: []float64{0.5 + offset[0], 128.9 + offset[1], -1 + offset[2]}},
			{Name: "rightmouth", Position: []float64{1.5 + offset[0], 128.9 + offset[1], -1 + offset[2]}},
		},
		{
			{Name: "neck", Position: []float64{1 + offset[0], 126.9 + offset[1], 0.5 + offset[2]}},
		},
		{
			{Name: "body1", Position: []float64{0 + offset[0], 125.9 + offset[1], 0 + offset[2]}},
			{Name: "body2", Position: []float64{1 + offset[0], 125.9 + offset[1], 0 + offset[2]}},
			{Name: "body3", Position: []float64{2 + offset[0], 125.9 + offset[1], 0 + offset[2]}},
			{Name: "body4", Position: []float64{0 + offset[0], 125.9 + offset[1], 1 + offset[2]}},
			{Name: "body5", Position: []float64{1 + offset[0], 125.9 + offset[1], 1 + offset[2]}},
			{Name: "body6", Position: []float64{2 + offset[0], 125.9 + offset[1], 1 + offset[2]}},
			{Name: "body7", Position: []float64{0 + offset[0], 124.9 + offset[1], 0 + offset[2]}},
			{Name: "body8", Position: []float64{1 + offset[0], 124.9 + offset[1], 0 + offset[2]}},
			{Name: "body9", Position: []float64{2 + offset[0], 124.9 + offset[1], 0 + offset[2]}},
			{Name: "body10", Position: []float64{0 + offset[0], 124.9 + offset[1], 1 + offset[2]}},
			{Name: "body11", Position: []float64{1 + offset[0], 124.9 + offset[1], 1 + offset[2]}},
			{Name: "body12", Position: []float64{2 + offset[0], 124.9 + offset[1], 1 + offset[2]}},
			{Name: "body13", Position: []float64{0 + offset[0], 125.9 + offset[1], 2 + offset[2]}},
			{Name: "body14", Position: []float64{1 + offset[0], 125.9 + offset[1], 2 + offset[2]}},
			{Name: "body15", Position: []float64{2 + offset[0], 125.9 + offset[1], 2 + offset[2]}},
			{Name: "body16", Position: []float64{0 + offset[0], 125.9 + offset[1], 3 + offset[2]}},
			{Name: "body17", Position: []float64{1 + offset[0], 125.9 + offset[1], 3 + offset[2]}},
			{Name: "body18", Position: []float64{2 + offset[0], 125.9 + offset[1], 3 + offset[2]}},
			{Name: "body19", Position: []float64{0 + offset[0], 124.9 + offset[1], 2 + offset[2]}},
			{Name: "body20", Position: []float64{1 + offset[0], 124.9 + offset[1], 2 + offset[2]}},
			{Name: "body21", Position: []float64{2 + offset[0], 124.9 + offset[1], 2 + offset[2]}},
			{Name: "body22", Position: []float64{0 + offset[0], 124.9 + offset[1], 3 + offset[2]}},
			{Name: "body23", Position: []float64{1 + offset[0], 124.9 + offset[1], 3 + offset[2]}},
			{Name: "body24", Position: []float64{2 + offset[0], 124.9 + offset[1], 3 + offset[2]}},
		},
		{
			{Name: "leftbackleg1", Position: []float64{2 + offset[0], 123.7 + offset[1], 3 + offset[2]}},
			{Name: "leftbackknee1", Position: []float64{2 + offset[0], 122.5 + offset[1], 3 + offset[2]}},
			{Name: "leftbackleg2", Position: []float64{2 + offset[0], 121.3 + offset[1], 3 + offset[2]}},
		},
		{
			{Name: "leftfrontleg1", Position: []float64{2 + offset[0], 123.7 + offset[1], 0 + offset[2]}},
			{Name: "leftfrontknee1", Position: []float64{2 + offset[0], 122.5 + offset[1], 0 + offset[2]}},
			{Name: "leftfrontleg2", Position: []float64{2 + offset[0], 121.3 + offset[1], 0 + offset[2]}},
		},
		{
			{Name: "rightbackleg1", Position: []float64{0 + offset[0], 123.7 + offset[1], 3 + offset[2]}},
			{Name: "rightbackknee1", Position: []float64{0 + offset[0], 122.5 + offset[1], 3 + offset[2]}},
			{Name: "rightbackleg2", Position: []float64{0 + offset[0], 121.3 + offset[1], 3 + offset[2]}},
		},
		{
			{Name: "rightfrontleg1", Position: []float64{0 + offset[0], 123.7 + offset[1], 0 + offset[2]}},
			{Name: "rightfrontknee1", Position: []float64{0 + offset[0], 122.5 + offset[1], 0 + offset[2]}},
			{Name: "rightfrontleg2", Position: []float64{0 + offset[0], 121.3 + offset[1], 0 + offset[2]}},
		},
		{
			{Name: "tail1", Position: []float64{1 + offset[0], 127.2 + offset[1], 3 + offset[2]}},
			{Name: "tail2", Position: []float64{1 + offset[0], 128.4 + offset[1], 3 + offset[2]}},
			{Name: "tail3", Position: []float64{1 + offset[0], 129.6 + offset[1], 3 + offset[2]}},
		},
	}

	var wg sync.WaitGroup
	for _, group := range cubeGroups {
		for _, cube := range group {
			wg.Add(1)
			go spawnCube(cube, &wg)
		}
	}
	wg.Wait()

	//linkCubes("head6_BASE", "leftmouth_BASE", "hinge", "jaw_joint_left")
	//linkCubes("head7_BASE", "rightmouth_BASE", "hinge", "jaw_joint_right")

	/*stiffenJoint("jaw_joint_left", map[string]float64{
		"bias":    0.9,
		"damping": 1.0,
	})*/

	//linkCubes("head6_BASE", "leftmouth_BASE", "hinge", "jaw_joint_left")
	//linkCubes("head7_BASE", "rightmouth_BASE", "hinge", "jaw_joint_right")

	linkCubes("tail1_BASE", "tail2_BASE", "hinge", "tail_joint1")
	linkCubes("tail2_BASE", "tail3_BASE", "hinge", "tail_joint2")

	// Apply stiffening to all joints.
	start := time.Now()
	stiffenAllJoints()
	duration := time.Since(start) // End timer
	fmt.Println("stiffenAllJoints Function took:", duration)

	start = time.Now()
	SingleTCPConnectionExamplestiffenAllJoints()
	duration = time.Since(start) // End timer
	fmt.Println("SingleTCPConnectionExamplestiffenAllJoints Function took:", duration)

	start = time.Now()
	SingleThreadedstiffenAllJoints()
	duration = time.Since(start) // End timer
	fmt.Println("SingleThreadedstiffenAllJoints Function took:", duration)

	fmt.Println("Spawned all cubes.")
	unfreezeAllCubes()

	fmt.Println("Waiting 3 seconds before despawning...")
	time.Sleep(3 * time.Second)

	despawnAllCubes()
	fmt.Println("Despawned all cubes.")
}
