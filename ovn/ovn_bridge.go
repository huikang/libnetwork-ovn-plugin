package ovn

import (
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/socketplane/libovsdb"
)

//  setupBridge If bridge does not exist create it.
func (d *Driver) initBridge(id string) error {
	bridgeName := d.networks[id].BridgeName
	if err := d.ovnnber.addBridge(bridgeName); err != nil {
		log.Errorf("error creating logical bridge [ %s ] : [ %s ]", bridgeName, err)
		return err
	}

	return nil
}

func (ovnnber *ovnnber) createBridgeIface(name string) error {
	err := ovnnber.createLogicalBridge(name)
	if err != nil {
		log.Errorf("Bridge creation failed for the bridge named [ %s ] with errors: %s", name, err)
	}
	return nil
}

// createOvsdbBridge creates the OVS bridge
func (ovnnber *ovnnber) createLogicalBridge(bridgeName string) error {
	namedBridgeUUID := "bridge"

	// Bridge row to insert
	bridge := make(map[string]interface{})
	bridge["name"] = bridgeName

	insertBridgeOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "Logical_Switch",
		Row:      bridge,
		UUIDName: namedBridgeUUID,
	}

	operations := []libovsdb.Operation{insertBridgeOp}
	reply, _ := ovnnber.ovsdb.Transact("OVN_Northbound", operations...)

	if len(reply) < len(operations) {
		return errors.New("Number of Replies should be atleast equal to number of Operations")
	}
	for i, o := range reply {
		if o.Error != "" && i < len(operations) {
			return errors.New("Transaction Failed due to an error :" + o.Error + " details : " + o.Details)
		} else if o.Error != "" {
			return errors.New("Transaction Failed due to an error :" + o.Error + " details : " + o.Details)
		}
	}

	log.Debugf("Created OVN logical bridge [ %s ]", bridgeName)
	return nil
}

// Check if port exists prior to creating a bridge
func (ovnnber *ovnnber) addBridge(bridgeName string) error {
	log.Debugf("Create OVN logical bridge [ %s ]", bridgeName)
	if ovnnber.ovsdb == nil {
		return errors.New("OVS not connected")
	}
	// If the bridge has been created, an internal port with the same name will exist
	exists, err := ovnnber.bridgeExists(bridgeName)
	if err != nil {
		return err
	}
	if !exists {
		if err := ovnnber.createBridgeIface(bridgeName); err != nil {
			return err
		}
		exists, err = ovnnber.bridgeExists(bridgeName)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("Error creating Bridge")
		}
	}
	return nil
}

func (d *Driver) addVethPort(bridgeName, vethOut, mac, portName, cnid string) error {
	if err := d.ovsdber.addOvsVethPort(bridgeName, vethOut, mac); err != nil {
		log.Errorf("error add ovs veth port [ %s %s ] on bridge [ %s ]", vethOut, mac, bridgeName)
		return err
	}

	if err := d.ovsdber.bindVeth(vethOut, mac, portName, cnid); err != nil {
		log.Errorf("error bind veth [ %s %s ] eid [ %s ] on bridge [ %s ]", vethOut, mac, portName, bridgeName)
		return err
	}
	return nil
}

func (d *Driver) createEndpoint(bridgeName, endpointName string) error {
	if err := d.ovnnber.addLogicalPort(bridgeName, endpointName); err != nil {
		log.Errorf("error creating logical port [ %s ] on bridge [ %s ] : [ %s ]", endpointName, bridgeName, err)
		return err
	}
	return nil
}

func (d *Driver) deleteEndpoint(bridgeName, logicalPortName string) error {
	if err := d.ovnnber.delLogicalPort(bridgeName, logicalPortName); err != nil {
		log.Errorf("error deleting logical port [ %s ] on bridge [ %s ] : [ %s ]", logicalPortName, bridgeName, err)
		return err
	}
	return nil
}

func (d *Driver) setEndpointAddr(logicalPortName, ipaddr, macaddr string) error {
	if err := d.ovnnber.setLogicalPortAddr(logicalPortName, ipaddr, macaddr); err != nil {
		log.Errorf("error set logical port [ %s ] to [ %s ] : [ %s ]", logicalPortName, ipaddr, macaddr)
		return err
	}
	return nil
}

func (ovnnber *ovnnber) delLogicalPort(switchName, logicalPortName string) error {
	log.Infof("ovnnber deleting port [ %s ] on switch [ %s ]", logicalPortName, switchName)

	// Achieve in two transactions:
	// 1. find the UUID of the logicalport in the Logical_Switch_Port table
	// 2. removing the uuid from the ports of that switch in the Logical_Switch table

	condition := libovsdb.NewCondition("name", "==", logicalPortName)
	selectOp := libovsdb.Operation{
		Op:    "select",
		Table: "Logical_Switch_Port",
		// fixmehk: can not use _uuid due to any issue with libovsdb
		// Columns: []string{"_uuid"},
		Where: []interface{}{condition},
	}

	operations := []libovsdb.Operation{selectOp}
	reply, _ := ovnnber.ovsdb.Transact("OVN_Northbound", operations...)

	if len(reply) < len(operations) {
		return errors.New("Number of Replies should be at least equal to number of Operations")
	}

	if reply[0].Error != "" {
		return errors.New("Transaction Failed due to an error :" + reply[0].Error + " details : " + reply[0].Details)
	}

	// fixmehk: libovsdb can not return the _uuid of the selected row
	//     see the issue of libovsdb:
	//     https://github.com/socketplane/libovsdb/issues/45
	portUUID := getRowUUID(reply[0].Rows[0])

	// deleting an endpoint in the ports row in Logical_Switch table requires
	mutateUUID := []libovsdb.UUID{
		{GoUUID: portUUID},
	}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("ports", "delete", mutateSet)
	condition = libovsdb.NewCondition("name", "==", switchName)

	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Logical_Switch",
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations = []libovsdb.Operation{mutateOp}
	reply, _ = ovnnber.ovsdb.Transact("OVN_Northbound", operations...)

	if len(reply) < len(operations) {
		log.Infof("uuid: %v", reply)
		return errors.New("Number of Replies should be at least equal to number of Operations")
	}

	if reply[0].Error != "" {
		return errors.New("Transaction Failed due to an error :" + reply[0].Error + " details : " + reply[0].Details)
	}

	return nil
}

func (ovnnber *ovnnber) setLogicalPortAddr(logicalPortName, ipaddr, macaddr string) error {
	ipmac := macaddr + " " + ipaddr
	mutateAddr := []string{ipmac}
	mutateSet, _ := libovsdb.NewOvsSet(mutateAddr)
	mutation := libovsdb.NewMutation("addresses", "insert", mutateSet)
	condition := libovsdb.NewCondition("name", "==", logicalPortName)

	// Mutate operation
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Logical_Switch_Port",
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{mutateOp}
	reply, _ := ovnnber.ovsdb.Transact("OVN_Northbound", operations...)

	if len(reply) < len(operations) {
		return errors.New("Number of Replies should be at least equal to number of Operations")
	}
	for i, o := range reply {
		if o.Error != "" && i < len(operations) {
			return errors.New("Transaction Failed due to an error :" + o.Error + " details : " + o.Details)
		} else if o.Error != "" {
			return errors.New("Transaction Failed due to an error :" + o.Error + " details : " + o.Details)
		}
	}

	return nil
}

// Check if port exists prior to creating a bridge
func (ovnnber *ovnnber) addLogicalPort(switchName, logicalPortName string) error {
	log.Infof("addlogicalPort [ %s ] to switch [ %s ]", logicalPortName, switchName)

	namedEndpointUUID := "endpoint"

	// Bridge row to insert
	port := make(map[string]interface{})
	port["name"] = logicalPortName
	port["type"] = ""
	port["up"] = false

	insertPortOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "Logical_Switch_Port",
		Row:      port,
		UUIDName: namedEndpointUUID,
	}

	// Inserting a row in Logical_Switch_Port table requires mutating the Logical_Switch table.
	mutateUUID := []libovsdb.UUID{
		{GoUUID: namedEndpointUUID},
	}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("ports", "insert", mutateSet)
	condition := libovsdb.NewCondition("name", "==", switchName)

	// Mutate operation
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Logical_Switch",
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{insertPortOp, mutateOp}
	reply, _ := ovnnber.ovsdb.Transact("OVN_Northbound", operations...)

	if len(reply) < len(operations) {
		return errors.New("Number of Replies should be at least equal to number of Operations")
	}
	for i, o := range reply {
		if o.Error != "" && i < len(operations) {
			return errors.New("Transaction Failed due to an error :" + o.Error + " details : " + o.Details)
		} else if o.Error != "" {
			return errors.New("Transaction Failed due to an error :" + o.Error + " details : " + o.Details)
		}
	}
	log.Debugf("Added logical port [ %s ] to logical switch [ %s ]", logicalPortName, switchName)

	return nil
}

func (ovsdber *ovsdber) bindVeth(vethOut, mac, portName, cnid string) error {
	log.Infof("bind veth [ %s %s ]", vethOut, portName)
	// 2. ovs_vsctl("set", "interface", veth_outside,
	//        "external_ids:attached-mac=" + mac_address,
	//        "external_ids:iface-id=" + eid,
	//        "external_ids:vm-id=" + vm_id,
	//        "external_ids:iface-status=active")

	gomap := make(map[interface{}]interface{})
	gomap["attached-map"] = mac
	gomap["iface-id"] = portName
	gomap["vm-id"] = cnid
	gomap["iface-status"] = "active"
	mutateMap, _ := libovsdb.NewOvsMap(gomap)
	mutation := libovsdb.NewMutation("external_ids", "insert", mutateMap)
	condition := libovsdb.NewCondition("name", "==", vethOut)

	// Mutate operation
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Interface",
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
	operations := []libovsdb.Operation{mutateOp}
	reply, _ := ovsdber.ovsdb.Transact("Open_vSwitch", operations...)

	if len(reply) < len(operations) {
		log.Error("Number of Replies should be atleast equal to number of Operations")
	}
	for i, o := range reply {
		if o.Error != "" && i < len(operations) {
			return errors.New("Transaction Failed due to an error :" + o.Error + " details : " + o.Details)
		} else if o.Error != "" {
			return errors.New("Transaction Failed due to an error :" + o.Error + " details : " + o.Details)
		}
	}
	return nil
}

func (ovsdber *ovsdber) deletePort(bridgeName string, portName string) error {
	log.Infof("ovsdb deleting port [ %s ] on switch [ %s ]", portName, bridgeName)

	// Achieve in two transactions:
	// 1. find the UUID of the port in the Port table
	// 2. removing the uuid from the Ports in Port and Bridge tables

	condition := libovsdb.NewCondition("name", "==", portName)
	selectOp := libovsdb.Operation{
		Op:    "select",
		Table: "Port",
		// fixmehk: can not use _uuid due to any issue with libovsdb
		// Columns: []string{"_uuid"},
		Where: []interface{}{condition},
	}

	operations := []libovsdb.Operation{selectOp}
	reply, _ := ovsdber.ovsdb.Transact("Open_vSwitch", operations...)

	if len(reply) < len(operations) {
		return errors.New("Number of Replies should be at least equal to number of Operations")
	}

	if reply[0].Error != "" {
		return errors.New("Transaction Failed due to an error :" + reply[0].Error + " details : " + reply[0].Details)
	}

	// fixmehk: libovsdb can not return the _uuid of the selected row
	//     see the issue of libovsdb:
	//     https://github.com/socketplane/libovsdb/issues/45
	portUUID := getRowUUID(reply[0].Rows[0])
	log.Infof("Port uuid %v", portUUID)

	condition = libovsdb.NewCondition("name", "==", portName)
	deleteOp := libovsdb.Operation{
		Op:    "delete",
		Table: "Port",
		Where: []interface{}{condition},
	}

	// fixmehk: Use ovsdb cache table
	// portUUID = portUUIDForName(portName)
	if portUUID == "" {
		log.Error("Unable to find a matching Port : ", portName)
		return fmt.Errorf("Unable to find a matching Port : [ %s ]", portName)
	}

	// Deleting a Bridge row in Bridge table requires mutating the open_vswitch table.
	mutateUUID := []libovsdb.UUID{
		{GoUUID: portUUID},
	}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("ports", "delete", mutateSet)
	condition = libovsdb.NewCondition("name", "==", bridgeName)

	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations = []libovsdb.Operation{deleteOp, mutateOp}
	reply, _ = ovsdber.ovsdb.Transact("Open_vSwitch", operations...)

	if len(reply) < len(operations) {
		log.Error("Number of Replies should be atleast equal to number of Operations")
		return fmt.Errorf("Number of Replies should be atleast equal to number of Operations")
	}
	for i, o := range reply {
		if o.Error != "" && i < len(operations) {
			log.Error("Transaction Failed due to an error :", o.Error, " in ", operations[i])
			return fmt.Errorf("Transaction Failed due to an error: %s in %v", o.Error, operations[i])
		} else if o.Error != "" {
			log.Error("Transaction Failed due to an error :", o.Error)
			return fmt.Errorf("Transaction Failed due to an error %s", o.Error)
		}
	}
	log.Infof("ovsdb deleted port %s", portName)
	return nil
}

func (ovsdber *ovsdber) addOvsVethPort(bridgeName, vethOut, mac string) error {
	// 1. ovs_vsctl("add-port", OVN_BRIDGE, veth_outside)
	fmt.Printf("Added port [ %s ] to switch [  %s ]", vethOut, bridgeName)

	namedPortUUID := "port"
	namedIntfUUID := "intf"

	// intf row to insert
	intf := make(map[string]interface{})
	intf["name"] = vethOut
	intf["type"] = `system`

	insertIntfOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "Interface",
		Row:      intf,
		UUIDName: namedIntfUUID,
	}

	// port row to insert
	port := make(map[string]interface{})
	port["name"] = vethOut
	port["interfaces"] = libovsdb.UUID{
		GoUUID: namedIntfUUID,
	}

	insertPortOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "Port",
		Row:      port,
		UUIDName: namedPortUUID,
	}
	// Inserting a row in Port table requires mutating the bridge table.
	mutateUUID := []libovsdb.UUID{
		{GoUUID: namedPortUUID},
	}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("ports", "insert", mutateSet)
	condition := libovsdb.NewCondition("name", "==", bridgeName)

	// Mutate operation
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
	operations := []libovsdb.Operation{insertIntfOp, insertPortOp, mutateOp}
	reply, _ := ovsdber.ovsdb.Transact("Open_vSwitch", operations...)

	if len(reply) < len(operations) {
		fmt.Println(len(reply))
		fmt.Println(reply)
		log.Error("Number of Replies should be atleast equal to number of Operations")
	}
	for i, o := range reply {
		if o.Error != "" && i < len(operations) {
			return errors.New("Transaction Failed due to an error :" + o.Error + " details : " + o.Details)
		} else if o.Error != "" {
			return errors.New("Transaction Failed due to an error :" + o.Error + " details : " + o.Details)
		}
	}

	return nil
}
