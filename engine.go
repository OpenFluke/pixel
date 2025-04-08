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

func setJointParams(conn net.Conn, jointName string, params map[string]float64) {
	cmd := Message{
		"type":       "set_joint_params",
		"joint_name": jointName,
		"params":     params,
	}
	if err := sendJSONMessage(conn, cmd); err != nil {
		fmt.Printf("[setJointParams] Failed to send joint params for %s: %v\n", jointName, err)
		return
	}
	resp, err := readResponse(conn)
	if err != nil {
		fmt.Printf("[setJointParams] Read error for %s: %v\n", jointName, err)
		return
	}
	fmt.Printf("[setJointParams] %s response: %s\n", jointName, resp)
}

func stiffenAllJointsBULK() {
	params := map[string]float64{
		"limit_upper":           0.0,
		"limit_lower":           0.0,
		"motor_enable":          1.0,
		"motor_target_velocity": 0.0,
		"motor_max_impulse":     1000.0,
	}

	var wg sync.WaitGroup
	for _, link := range globalCubeLinks {
		wg.Add(1)
		go func(joint CubeLink) {
			defer wg.Done()

			conn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Printf("[stiffenAllJoints] Failed to connect for joint %s: %v\n", joint.JointName, err)
				return
			}
			defer conn.Close()

			if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
				fmt.Printf("[stiffenAllJoints] Auth write error for joint %s: %v\n", joint.JointName, err)
				return
			}
			if _, err := readResponse(conn); err != nil {
				fmt.Printf("[stiffenAllJoints] Auth response error for joint %s: %v\n", joint.JointName, err)
				return
			}

			setJointParams(conn, joint.JointName, params)
		}(link)
	}
	wg.Wait()
	fmt.Println("[stiffenAllJoints] All joints updated.")
}

func setMouthColorYellow() {
	var mouthCubes = []string{"leftmouth_BASE",
		"rightmouth_BASE", "body24_BASE", "body22_BASE", "body9_BASE", "body7_BASE",
		"body17_BASE",
		"head1_BASE",
		"head3_BASE",
		"head2_BASE",
		"head8_BASE",
		"body2_BASE",
	}

	var wg sync.WaitGroup
	for _, cubeName := range mouthCubes {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			conn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Printf("[Color] Failed to connect for cube %s: %v\n", name, err)
				return
			}
			defer conn.Close()

			if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
				fmt.Printf("[Color] Auth write error for cube %s: %v\n", name, err)
				return
			}
			if _, err := readResponse(conn); err != nil {
				fmt.Printf("[Color] Auth response error for cube %s: %v\n", name, err)
				return
			}

			colorMsg := Message{
				"type":      "set_color",
				"cube_name": name,
				"hex":       "#FFFF00", // Yellow
			}

			if err := sendJSONMessage(conn, colorMsg); err != nil {
				fmt.Printf("[Color] Failed to send color change for cube %s: %v\n", name, err)
			}
		}(cubeName)
	}
	wg.Wait()
}

// testLinkBodyCubes creates a TCP connection, authenticates, and sends a JSON command
// to link all cubes whose names start with the given prefix.
// It prints both the authentication response and the command response.
func testLinkBodyCubes(prefix string, jointType string, jointParams map[string]float64) {
	// Connect to the server.
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("[testLinkBodyCubes] Error connecting:", err)
		return
	}
	defer conn.Close()

	// Authenticate with the server.
	if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
		fmt.Println("[testLinkBodyCubes] Auth write error:", err)
		return
	}
	authResp, err := readResponse(conn)
	if err != nil {
		fmt.Println("[testLinkBodyCubes] Failed to read auth response:", err)
		return
	}
	fmt.Println("[testLinkBodyCubes] Auth response:", authResp)

	// Build the JSON command message.
	cmdMsg := Message{
		"type":         "link_body_cubes",
		"prefix":       prefix,
		"joint_type":   jointType,
		"joint_params": jointParams,
	}

	// Send the command.
	if err := sendJSONMessage(conn, cmdMsg); err != nil {
		fmt.Println("[testLinkBodyCubes] Error sending command:", err)
		return
	}

	// Optionally read the server's response to the command.
	cmdResp, err := readResponse(conn)
	if err != nil {
		fmt.Println("[testLinkBodyCubes] Error reading command response:", err)
		return
	}
	fmt.Println("[testLinkBodyCubes] Command response:", cmdResp)
}

func linkCubeChains(chains [][]string, jointType string, jointParams map[string]float64) error {
	// Establish TCP connection
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return fmt.Errorf("[linkCubeChains] Failed to connect: %v", err)
	}
	defer conn.Close()

	// Authenticate
	if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
		return fmt.Errorf("[linkCubeChains] Auth write error: %v", err)
	}
	authResp, err := readResponse(conn)
	if err != nil {
		return fmt.Errorf("[linkCubeChains] Failed to read auth response: %v", err)
	}
	fmt.Println("[linkCubeChains] Auth response:", authResp)

	// Construct the command
	cmd := Message{
		"type":         "link_cube_chains",
		"chains":       chains,
		"joint_type":   jointType,
		"joint_params": jointParams,
	}

	// Send the command
	if err := sendJSONMessage(conn, cmd); err != nil {
		return fmt.Errorf("[linkCubeChains] Failed to send command: %v", err)
	}

	// Read response (optional)
	resp, err := readResponse(conn)
	if err != nil {
		return fmt.Errorf("[linkCubeChains] Error reading response: %v", err)
	}
	fmt.Println("[linkCubeChains] Server response:", resp)

	// Update globalCubeLinks for tracking (optional, adjust as needed)
	linkListMutex.Lock()
	defer linkListMutex.Unlock()
	for _, chain := range chains {
		for i := 0; i < len(chain)-1; i++ {
			cubeA := chain[i]
			cubeB := chain[i+1]
			jointName := fmt.Sprintf("joint_%s_%s_%s", jointType, cubeA, cubeB) // Simplified name
			globalCubeLinks = append(globalCubeLinks, CubeLink{
				JointName: jointName,
				CubeA:     cubeA,
				CubeB:     cubeB,
			})
		}
	}

	return nil
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

	setMouthColorYellow()

	//linkCubes("head6_BASE", "leftmouth_BASE", "hinge", "jaw_joint_left")
	//linkCubes("head7_BASE", "rightmouth_BASE", "hinge", "jaw_joint_right")

	/*stiffenJoint("jaw_joint_left", map[string]float64{
		"bias":    0.9,
		"damping": 1.0,
	})*/

	//linkCubes("head6_BASE", "leftmouth_BASE", "hinge", "jaw_joint_left")
	//linkCubes("head7_BASE", "rightmouth_BASE", "hinge", "jaw_joint_right")

	/*linkCubes("tail1_BASE", "tail2_BASE", "hinge", "tail_joint1")
	linkCubes("tail2_BASE", "tail3_BASE", "hinge", "tail_joint2")

	//backlegs
	linkCubes("leftbackleg1_BASE", "leftbackknee1_BASE", "hinge", "backleft_leg_joint1")
	linkCubes("leftbackknee1_BASE", "leftbackleg2_BASE", "hinge", "backleft_leg_joint2")

	linkCubes("rightbackleg1_BASE", "rightbackknee1_BASE", "hinge", "backright_leg_joint1")
	linkCubes("rightbackknee1_BASE", "rightbackleg2_BASE", "hinge", "backright_leg_joint2")

	//frontlegs
	linkCubes("leftfrontleg1_BASE", "leftfrontknee1_BASE", "hinge", "frontleft_leg_joint1")
	linkCubes("leftfrontknee1_BASE", "leftfrontleg2_BASE", "hinge", "frontleft_leg_joint2")

	linkCubes("rightfrontleg1_BASE", "rightfrontknee1_BASE", "hinge", "frontright_leg_joint1")
	linkCubes("rightfrontknee1_BASE", "rightfrontleg2_BASE", "hinge", "frontright_leg_joint2")

	//connecting legs to body
	linkCubes("body24_BASE", "leftbackleg1_BASE", "hinge", "backlefttobody_leg_joint1")
	linkCubes("body22_BASE", "rightbackleg1_BASE", "hinge", "backrighttobody_leg_joint1")
	linkCubes("body9_BASE", "leftfrontleg1_BASE", "hinge", "frontlefttobody_leg_joint1")
	linkCubes("body7_BASE", "rightfrontleg1_BASE", "hinge", "frontrighttobody_leg_joint1")
	*/
	// Apply stiffening to all joints.
	/*start := time.Now()
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
	fmt.Println("SingleThreadedstiffenAllJoints Function took:", duration)*/

	start := time.Now()
	stiffenAllJointsBULK()
	duration := time.Since(start) // End timer
	fmt.Println("stiffenAllJointsBULK Function took:", duration)

	// Define joint parameters for the hinge joint.
	jointParams := map[string]float64{
		"limit_upper":           0.0,
		"limit_lower":           0.0,
		"motor_enable":          1.0,
		"motor_target_velocity": 0.0,
		"motor_max_impulse":     1000.0,
	}

	// Call testLinkBodyCubes with prefix "body" and joint type "hinge".
	testLinkBodyCubes("body", "hinge", jointParams)

	testLinkBodyCubes("head", "hinge", jointParams)

	chains := [][]string{
		{"leftbackleg1_BASE", "leftbackknee1_BASE", "leftbackleg2_BASE"},
		{"rightbackleg1_BASE", "rightbackknee1_BASE", "rightbackleg2_BASE"},
		{"leftfrontleg1_BASE", "leftfrontknee1_BASE", "leftfrontleg2_BASE"},
		{"rightfrontleg1_BASE", "rightfrontknee1_BASE", "rightfrontleg2_BASE"},
		{"tail1_BASE", "tail2_BASE", "tail3_BASE"},
		{"body24_BASE", "leftbackleg1_BASE"},
		{"body22_BASE", "rightbackleg1_BASE"},
		{"body9_BASE", "leftfrontleg1_BASE"},
		{"body7_BASE", "rightfrontleg1_BASE"},
		{"body17_BASE", "tail3_BASE"},
		{"head1_BASE", "rightear_BASE"},
		{"head3_BASE", "leftear_BASE"},
		{"head2_BASE", "leftmouth_BASE"},
		{"head2_BASE", "rightmouth_BASE"},
		{"head8_BASE", "neck_BASE", "body2_BASE"},
	}

	if err := linkCubeChains(chains, "hinge", jointParams); err != nil {
		fmt.Println("Error linking cube chains:", err)
	}

	/*groups := [][]string{
		{"body24_BASE", "leftbackleg1_BASE", "leftbackknee1_BASE", "leftbackleg2_BASE"},
		{"body22_BASE", "rightbackleg1_BASE", "rightbackknee1_BASE", "rightbackleg2_BASE"},
		//{"body24_BASE", "leftbackleg1_BASE", "leftbackknee1_BASE", "leftbackleg2_BASE"},
		//{"body24_BASE", "leftbackleg1_BASE", "leftbackknee1_BASE", "leftbackleg2_BASE"},
	}*/

	//linkCubeGroups(groups, "hinge", jointParams)

	fmt.Println("Spawned all cubes.")
	unfreezeAllCubes()

	//rotateLegDemo("joint_hinge_leftbackknee1_BASE_leftbackleg2_BASE")

	/*fmt.Println("➡️ Rotating leftbackleg2_BASE +90° Y")
	rotateCube("leftbackleg2_BASE", []float64{0, 90, 0})
	time.Sleep(2 * time.Second)

	fmt.Println("⬅️ Rotating leftbackleg2_BASE -90° Y")
	rotateCube("leftbackleg2_BASE", []float64{0, -90, 0})*/

	// 🔎 Find joint for leftbackleg2_BASE and animate it
	/*joint := findClosestJoint("leftbackleg2_BASE")
	if joint != "" {
		rotateLegDemo(joint)

		// Wait and try rotating on another axis by reversing again
		time.Sleep(1 * time.Second)
		fmt.Println("⏩ Rotating again in new direction...")
		conn, _ := net.Dial("tcp", serverAddr)
		defer conn.Close()
		conn.Write([]byte(authPass + delimiter))
		readResponse(conn)

		setJointParam(conn, joint, "motor_target_velocity", 0.0)
		setJointParam(conn, joint, "motor_enable", 1.0)
		setJointParam(conn, joint, "motor_target_velocity", 5.0)
		time.Sleep(500 * time.Millisecond)
		setJointParam(conn, joint, "motor_target_velocity", -5.0)
		time.Sleep(500 * time.Millisecond)
		setJointParam(conn, joint, "motor_target_velocity", 0.0)
	}*/

	/*rotateAllJointsForCube("leftbackleg1_BASE")
	rotateAllJointsForCube("leftbackknee1_BASE")
	rotateAllJointsForCube("leftbackleg2_BASE")*/

	//rotateCubeJoints("leftbackleg1_BASE", 2.5, 1*time.Second)

	for i := 0; i < 5; i++ {
		rotateCubeJoints("tail3_BASE", -3.0, 800*time.Millisecond)
		time.Sleep(2 * time.Second)
	}

	fmt.Println("Waiting 3 seconds before despawning...")
	time.Sleep(3 * time.Second)

	despawnAllCubes()
	fmt.Println("Despawned all cubes.")
}

func findClosestJoint(targetCube string) string {
	linkListMutex.Lock()
	defer linkListMutex.Unlock()

	for _, link := range globalCubeLinks {
		if link.CubeA == targetCube || link.CubeB == targetCube {
			fmt.Printf("🔍 Found joint: %s (%s <-> %s)\n", link.JointName, link.CubeA, link.CubeB)
			return link.JointName
		}
	}
	fmt.Printf("⚠️ No joint found for cube: %s\n", targetCube)
	return ""
}

func rotateLegDemo(jointName string) {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Printf("[rotateLegDemo] Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
		fmt.Println("[rotateLegDemo] Auth write error:", err)
		return
	}
	if _, err := readResponse(conn); err != nil {
		fmt.Println("[rotateLegDemo] Auth response error:", err)
		return
	}

	// First 90-degree rotation (approx via velocity)
	fmt.Println("↪️ Rotating forward...")
	setJointParam(conn, jointName, "motor_enable", 1)
	setJointParam(conn, jointName, "motor_target_velocity", 5.0) // adjust speed as needed
	setJointParam(conn, jointName, "motor_max_impulse", 1000)

	time.Sleep(1 * time.Second) // Let it move for a second

	// Reverse direction
	fmt.Println("↩️ Rotating back...")
	setJointParam(conn, jointName, "motor_target_velocity", -5.0)

	time.Sleep(1 * time.Second)

	// Stop movement
	setJointParam(conn, jointName, "motor_target_velocity", 0.0)
	fmt.Println("🛑 Leg motion complete.")
}

func rotateCube(cubeName string, rotationDelta []float64) {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Printf("[rotateCube] Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
		fmt.Println("[rotateCube] Auth write error:", err)
		return
	}
	if _, err := readResponse(conn); err != nil {
		fmt.Println("[rotateCube] Auth response error:", err)
		return
	}

	cmd := Message{
		"type":   "apply_force",
		"rotate": rotationDelta, // in degrees
		"target": cubeName,      // optional, in case your server supports targeted cube
	}

	if err := sendJSONMessage(conn, cmd); err != nil {
		fmt.Printf("[rotateCube] Failed to send rotate command: %v\n", err)
		return
	}

	resp, err := readResponse(conn)
	if err != nil {
		fmt.Printf("[rotateCube] Failed to read rotate response: %v\n", err)
		return
	}

	fmt.Printf("[rotateCube] Server response: %s\n", resp)
}

func rotateAllJointsForCube(targetCube string) {
	fmt.Printf("🐾 Brute-forcing all joints for cube: %s\n", targetCube)

	for _, link := range globalCubeLinks {
		if link.CubeA == targetCube || link.CubeB == targetCube {
			fmt.Printf("➡️ Rotating joint: %s (%s <-> %s)\n", link.JointName, link.CubeA, link.CubeB)

			conn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Printf("[rotateAllJointsForCube] Failed to connect for joint %s: %v\n", link.JointName, err)
				continue
			}

			if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
				conn.Close()
				continue
			}
			_, _ = readResponse(conn)

			setJointParam(conn, link.JointName, "motor_enable", 1.0)
			setJointParam(conn, link.JointName, "motor_target_velocity", 5.0)
			setJointParam(conn, link.JointName, "motor_max_impulse", 1000.0)

			time.Sleep(500 * time.Millisecond)

			setJointParam(conn, link.JointName, "motor_target_velocity", -5.0)

			time.Sleep(500 * time.Millisecond)

			setJointParam(conn, link.JointName, "motor_target_velocity", 0.0)

			conn.Close()
			fmt.Printf("✅ Done rotating joint: %s\n", link.JointName)
		}
	}
}

func getJointsForCube(cubeName string) []string {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("[getJointsForCube] Failed to connect:", err)
		return nil
	}
	defer conn.Close()

	// Authenticate
	if _, err := conn.Write([]byte(authPass + delimiter)); err != nil {
		fmt.Println("[getJointsForCube] Auth write error:", err)
		return nil
	}
	if _, err := readResponse(conn); err != nil {
		fmt.Println("[getJointsForCube] Auth response error:", err)
		return nil
	}

	// Send command
	cmd := Message{
		"type":      "get_joints_for_cube",
		"cube_name": cubeName,
	}
	if err := sendJSONMessage(conn, cmd); err != nil {
		fmt.Println("[getJointsForCube] Failed to send command:", err)
		return nil
	}

	respRaw, err := readResponse(conn)
	if err != nil {
		fmt.Println("[getJointsForCube] Failed to read response:", err)
		return nil
	}

	var resp struct {
		Type     string   `json:"type"`
		CubeName string   `json:"cube_name"`
		Joints   []string `json:"joints"`
	}
	if err := json.Unmarshal([]byte(respRaw), &resp); err != nil {
		fmt.Println("[getJointsForCube] JSON unmarshal failed:", err)
		return nil
	}

	return resp.Joints
}

func rotateCubeJoints(cubeName string, velocity float64, duration time.Duration) {
	joints := getJointsForCube(cubeName)
	if len(joints) == 0 {
		fmt.Printf("[rotateCubeJoints] No joints found for cube %s\n", cubeName)
		return
	}

	fmt.Printf("🦴 Found %d joints for %s. Applying rotation...\n", len(joints), cubeName)

	for _, joint := range joints {
		go func(jn string) {
			conn, err := net.Dial("tcp", serverAddr)
			if err != nil {
				fmt.Printf("[rotateCubeJoints] Connect failed: %v\n", err)
				return
			}
			defer conn.Close()

			// Authenticate
			conn.Write([]byte(authPass + delimiter))
			readResponse(conn)

			// Enable motor
			params := Message{
				"type":       "set_joint_params",
				"joint_name": jn,
				"params": map[string]float64{
					"motor_enable":          1.0,
					"motor_target_velocity": velocity,
					"motor_max_impulse":     500.0,
				},
			}
			sendJSONMessage(conn, params)
			readResponse(conn) // optional

			// Reverse after delay
			time.Sleep(duration)

			// Reverse
			params["params"].(map[string]float64)["motor_target_velocity"] = -velocity
			sendJSONMessage(conn, params)
			readResponse(conn)

			// Stop after another duration
			time.Sleep(duration)
			params["params"].(map[string]float64)["motor_target_velocity"] = 0.0
			sendJSONMessage(conn, params)
			readResponse(conn)

			fmt.Printf("↩️ Completed joint cycle for: %s\n", jn)
		}(joint)
	}
}
