package ovn

import (
	"errors"

	log "github.com/Sirupsen/logrus"
	"github.com/socketplane/libovsdb"
)

//  setupBridge If bridge does not exist create it.
func (d *Driver) initBridge(id string) error {
	log.Infof("initBridge id: %s\n", id)
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

	log.Infof("createLogcialBridge %s\n", bridgeName)

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
	return nil
}

// Check if port exists prior to creating a bridge
func (ovnnber *ovnnber) addBridge(bridgeName string) error {
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

	log.Infof("Added port")

	return nil
}
