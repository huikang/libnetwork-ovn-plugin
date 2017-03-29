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
