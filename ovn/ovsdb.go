package ovn

import (
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/socketplane/libovsdb"
)

const (
	ovsdbPort = 6640
	ovnNBPort = 6641
)

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

// getRowUUID extracts the uuid of the input row
func getRowUUID(columns map[string]interface{}) (uuid string) {
	// uuid has fixed format: e.g., [uuid fdfb4bdd-08ee-453e-849e-8ef8d2116a82]
	u := columns["_uuid"].([]interface{})
	k := u[1].(string)
	return k
}
