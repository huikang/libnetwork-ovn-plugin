package ovn

import (
	"errors"
	"fmt"
	"reflect"

	log "github.com/Sirupsen/logrus"
	"github.com/socketplane/libovsdb"
)

var (
	quit       chan bool
	update     chan *libovsdb.TableUpdates
	ovnnbCache map[string]map[string]libovsdb.Row
)

//  setupBridge If bridge does not exist create it.
func (d *Driver) initBridge(id string) error {
	bridgeName := d.networks[id].BridgeName
	if err := d.ovnnber.addBridge(bridgeName, id); err != nil {
		log.Errorf("error creating logical bridge [ %s ] : [ %s ]", bridgeName, err)
		return err
	}

	return nil
}

func (ovnnber *ovnnber) bridgeExists(portName string) (bool, error) {
	condition := libovsdb.NewCondition("name", "==", portName)
	selectOp := libovsdb.Operation{
		Op:    "select",
		Table: "Logical_Switch",
		Where: []interface{}{condition},
	}
	operations := []libovsdb.Operation{selectOp}
	reply, _ := ovnnber.ovsdb.Transact("OVN_Northbound", operations...)

	if len(reply) < len(operations) {
		return false, errors.New("Number of Replies should be atleast equal to number of Operations")
	}

	if reply[0].Error != "" {
		errMsg := fmt.Sprintf("Transaction Failed due to an error: %v", reply[0].Error)
		return false, errors.New(errMsg)
	}

	if len(reply[0].Rows) == 0 {
		return false, nil
	}
	return true, nil
}

func (ovnnber *ovnnber) endpointpointExists(endpointName string) (bool, error) {
	condition := libovsdb.NewCondition("name", "==", endpointName)
	selectOp := libovsdb.Operation{
		Op:    "select",
		Table: "Logical_Switch_Port",
		Where: []interface{}{condition},
	}
	operations := []libovsdb.Operation{selectOp}
	reply, _ := ovnnber.ovsdb.Transact("OVN_Northbound", operations...)

	if len(reply) < len(operations) {
		return false, errors.New("Number of Replies should be atleast equal to number of Operations")
	}

	if reply[0].Error != "" {
		errMsg := fmt.Sprintf("Transaction Failed due to an error: %v", reply[0].Error)
		return false, errors.New(errMsg)
	}

	if len(reply[0].Rows) == 0 {
		return false, nil
	}
	return true, nil
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

// createOvsdbBridge creates the OVS bridge
func (ovnnber *ovnnber) createLogicalBridge(bridgeName, netid string) error {
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

	// Set the net-id of the logical switch to netid
	gomap := make(map[interface{}]interface{})
	gomap["net-id"] = netid
	mutateMap, _ := libovsdb.NewOvsMap(gomap)
	mutation := libovsdb.NewMutation("external_ids", "insert", mutateMap)
	condition := libovsdb.NewCondition("name", "==", bridgeName)

	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Logical_Switch",
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{insertBridgeOp, mutateOp}
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
func (ovnnber *ovnnber) addBridge(bridgeName, netid string) error {
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
		if err := ovnnber.createLogicalBridge(bridgeName, netid); err != nil {
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

// OvnnbNotifier implements libovsdb.NotificationHandler interface
type OvnnbNotifier struct {
}

// Update Notification
func (o OvnnbNotifier) Update(context interface{}, tableUpdates libovsdb.TableUpdates) {
	populateCache(tableUpdates)
	update <- &tableUpdates
}

// Disconnected notification
func (o OvnnbNotifier) Disconnected(ovsClient *libovsdb.OvsdbClient) {
}

//Locked Notification
func (o OvnnbNotifier) Locked([]interface{}) {
}

// Stolen Notification
func (o OvnnbNotifier) Stolen([]interface{}) {
}

// Echo Notification
func (o OvnnbNotifier) Echo([]interface{}) {
}

func (ovnnber *ovnnber) initDBCache() {
	quit = make(chan bool)
	update = make(chan *libovsdb.TableUpdates)
	ovnnbCache = make(map[string]map[string]libovsdb.Row)

	// Register for ovsdb table notifications
	var notifier OvnnbNotifier
	ovnnber.ovsdb.Register(notifier)
	// Populate ovnnb cache for the default OVN_Northbound db
	initCache, err := ovnnber.ovsdb.MonitorAll("OVN_Northbound", "")
	if err != nil {
		log.Errorf("Error populating initial OVNNB cache: %s", err)
	}

	populateCache(*initCache)

	// async monitoring of the ovs bridge(s) for table updates
	go ovnnber.monitorLogicalSwitches()
	/*
		for ovnnber.getRootUUID() == "" {
			time.Sleep(time.Second * 1)
		}*/
}

func (ovnnber *ovnnber) monitorLogicalSwitches() {
	for {
		select {
		case currUpdate := <-update:
			for table, tableUpdate := range currUpdate.Updates {
				if table == "Logical_Switch" {
					for _, row := range tableUpdate.Rows {
						empty := libovsdb.Row{}
						if !reflect.DeepEqual(row.New, empty) {
							oldRow := row.Old
							newRow := row.New
							if _, ok := oldRow.Fields["name"]; !ok {
								name := newRow.Fields["name"].(string)
								externalIds := newRow.Fields["external_ids"].(libovsdb.OvsMap)
								netid := externalIds.GoMap["net-id"].(string)
								d := ovnnber.driver
								if _, ok := d.networks[netid]; !ok {
									log.Debugf("  netid [ %s ] created remotely", netid)
									d.netmu.Lock()
									d.networks[netid] = &NetworkState{
										id:         netid,
										BridgeName: name,
									}
									d.netmu.Unlock()
								}
							}
						}
					}
				}
			}
		}
	}
}

func (ovnnber *ovnnber) getRootUUID() string {
	for uuid := range ovnnbCache["OVN_Northbound"] {
		return uuid
	}
	return ""
}

func populateCache(updates libovsdb.TableUpdates) {
	for table, tableUpdate := range updates.Updates {
		if _, ok := ovnnbCache[table]; !ok {
			ovnnbCache[table] = make(map[string]libovsdb.Row)
		}
		for uuid, row := range tableUpdate.Rows {
			empty := libovsdb.Row{}
			if !reflect.DeepEqual(row.New, empty) {
				ovnnbCache[table][uuid] = row.New
			} else {
				delete(ovnnbCache[table], uuid)
			}
		}
	}
}
