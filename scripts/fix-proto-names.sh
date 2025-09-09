#!/bin/bash
set -eu

for FNAME in $@ ; do
    if [[ ! "$FNAME" =~ \.pb\.go$ ]] ; then
        continue
    fi

    sed -i "$FNAME" \
        -e 's/PrimaryGpu/PrimaryGPU/g' \
        -e 's/ContextId/ContextID/g' \
        -e 's/Cpu/CPU/g' \
        -e 's/Pid/PID/g' \
        -e 's/CloudinitDrive/CloudInitDrive/g' \
        -e 's/Hostpci/HostPCI/g' \
        -e 's/PciAddr/PCIAddr/g' \
        -e 's/PortId/PortID/g' \
        -e 's/NbdPort/NBDPort/g' \
        -e 's/WsPort/WSPort/g' \
        -e 's/VlanId/VlanID/g' \
        -e 's/Vni/VNI/g' \
        -e 's/Mtu/MTU/g' \
        -e 's/Ipnets/IPNets/g' \
        -e 's/IpfabricAttrs/IPFabricAttrs/g' \
        -e 's/ProcessId/ProcessID/g' \
        -e 's/TaskId/TaskID/g'

done

exit 0
