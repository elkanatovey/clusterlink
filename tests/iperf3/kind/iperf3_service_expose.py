#!/usr/bin/env python3
import os,time
import subprocess as sp
import sys
import argparse

proj_dir = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname( os.path.abspath(__file__)))))
sys.path.insert(0,f'{proj_dir}')

from tests.utils.mbgAux import runcmd, runcmdb, printHeader, getPodName
from tests.utils.kind.kindAux import useKindCluster

def exposeService(mbgName, mbgctlName, destSvc):
    useKindCluster(mbgName)
    mbgctlPod = getPodName("mbgctl")
    printHeader(f"\n\nStart exposing {destSvc} service to {mbgName}")
    runcmd(f'kubectl exec -i {mbgctlPod} -- ./mbgctl expose --service {destSvc}')


def bindService(mbgName, destSvc, port):
    useKindCluster(mbgName)
    mbgctlPod = getPodName("mbgctl")
    printHeader(f"\n\nStart binding {destSvc} service to {mbgName}")
    runcmd(f'kubectl exec -i {mbgctlPod} -- ./mbgctl add binding --service {destSvc} --port {port}')
############################### MAIN ##########################
if __name__ == "__main__":
    #parameters 
    mbg2Name     = "mbg2"
    mbgctl2Name  = "mbgctl2"
    destSvc      = "iperf3-server"
    
        
    
    print(f'Working directory {proj_dir}')
    os.chdir(proj_dir)

    exposeService(mbg2Name, mbgctl2Name, destSvc)
    

    