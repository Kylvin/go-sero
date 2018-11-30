#!/usr/bin

ROOT=$(cd `dirname $0`; pwd)
if [ -f "${ROOT}/pid" ];then
    kill -9 `cat pid`
fi

function sysname() {

    SYSTEM=`uname -s`
    if [[ "Darwin" == "$SYSTEM" ]]
    then
        echo "Darwin"
    fi

    if [[ "Linux" == "$SYSTEM" ]]
    then
        name=`cat /etc/system-release|awk '{print $1}'`
        echo "Linux $name"
    fi
}

SNAME=`sysname`
if [[ "Darwin" = "$SNAME" ]];then
    export DYLD_LIBRARY_PATH=${ROOT}/czero/lib/
    echo $DYLD_LIBRARY_PATH
else
    export LD_LIBRARY_PATH=${ROOT}/czero/lib/
    echo $LD_LIBRARY_PATH
fi


DEFAULT_DATD_DIR="${ROOT}/data"
LOGDIR="${ROOT}/log"
DEFAULT_RPCPORT=8545
DEFAULT_PORT=60602

cmd="${ROOT}/bin/gero"
if [[ $# -gt 0 ]]; then
     while [[ "$1" != "" ]]; do
       	 case "$1" in
		--datadir)
		    cmd="$cmd --datadir=$2";shift 2;;
        --dev)
		    cmd="$cmd --dev";shift;;

        --alpha)
		    cmd="$cmd --alpha";shift;;
        --rpc)
		    localhost=$(hostname -I|awk -F ' ' '{print $1}')
		    cmd="$cmd --rpc --rpcport $2 --rpcaddr $localhost --rpcapi 'personal,sero,web3' --rpccorsdomain '*'";shift;;
        --port)
		    cmd="$cmd --port $2";shift 2;;
		*)exit;;
        esac
    done
fi

if [[ ! "$cmd" == "* --datadir*" ]]; then
     cmd="$cmd --datadir=${DEFAULT_DATD_DIR}"
fi

if [[ ! "$cmd" == "* --port*" ]]; then
     cmd="$cmd --port ${DEFAULT_PORT}"
fi

echo $cmd
${cmd} &> ${ROOT}/log/gero.log & echo $! > ${ROOT}/pid