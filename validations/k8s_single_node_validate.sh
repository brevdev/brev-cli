#!/bin/bash

set +e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LOG_TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
SCRIPT_START_TIME=$(date +%s)
LOG_FILE="k8s_validation_${LOG_TIMESTAMP}.log"
CSV_FILE="k8s_validation_${LOG_TIMESTAMP}.csv"

TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0
WARNINGS=0
RESULTS=()

MICROK8S_STATUS=""
KUBECTL_VERSION=""
HELM_VERSION=""
CLUSTER_INFO_STATUS=""
NODES_READY=""
PODS_ACCESSIBLE=""
KUBECONFIG_EXISTS=""
DASHBOARD_ACCESSIBLE=""
DASHBOARD_TOKEN_EXISTS=""
DOCKER_STATUS=""
NVIDIA_DRIVER_VERSION=""
NVIDIA_CTK_VERSION=""
GPU_OPERATOR_STATUS=""
ERROR_MESSAGE=""


touch "$LOG_FILE" 2>/dev/null || {
    echo "Error: Cannot create log file: $LOG_FILE" >&2
    exit 1
}

{
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] [INFO] Kubernetes Installation Validation Script Started"
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] [INFO] Log file: $LOG_FILE"
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] [INFO] CSV file: $CSV_FILE"
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] [INFO] Timestamp: $TIMESTAMP"
} >> "$LOG_FILE"

_log_to_file() {
    local level="$1"
    local message="$2"
    local timestamp=$(date +"%Y-%m-%d %H:%M:%S")
    echo "[${timestamp}] [${level}] ${message}" >> "$LOG_FILE"
}

log_info() {
    _log_to_file "INFO" "$1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} ${1}" >&2
    _log_to_file "PASS" "$1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} ${1}" >&2
    _log_to_file "FAIL" "$1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} ${1}" >&2
    _log_to_file "WARN" "$1"
}

log_plain() {
    echo "$1" | tee -a "$LOG_FILE"
}

record_result() {
    local check_name="$1"
    local status="$2"
    local message="${3:-}"
    local details="${4:-}"
    
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    case "$status" in
        "PASS")
            PASSED_CHECKS=$((PASSED_CHECKS + 1))
            log_success "$check_name: $message"
            ;;
        "FAIL")
            FAILED_CHECKS=$((FAILED_CHECKS + 1))
            log_error "$check_name: $message"
            ;;
        "WARN")
            WARNINGS=$((WARNINGS + 1))
            log_warning "$check_name: $message"
            ;;
    esac
    
    RESULTS+=("$check_name|$status|$message|$details")
}

check_microk8s_installed() {
    
    log_info "Checking if MicroK8s is installed..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: microk8s is installed"
    if command -v microk8s &>/dev/null; then
        log_success "MicroK8s is installed"
        microk8s version 2>&1 | head -1 | tee -a "$LOG_FILE"
        return 0
    else
        log_error "MicroK8s is not installed"
        return 1
    fi
}

check_microk8s_running() {
    log_info "Checking MicroK8s status..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: microk8s status"

    # Run once and reuse output
    local status_output
    if ! status_output=$(sudo microk8s status 2>&1); then
        record_result "microk8s_running" "FAIL" "MicroK8s is not running"
        MICROK8S_STATUS="ERROR"
        ERROR_MESSAGE="microk8s status failed"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ microk8s status: FAILED"
        return 1
    fi

    # Check if running
    if echo "$status_output" | grep -qi "microk8s is running"; then
        record_result "microk8s_running" "PASS" "MicroK8s is running" "$status_output"
        MICROK8S_STATUS="OK"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ microk8s status: OK"
    else
        record_result "microk8s_running" "WARN" "MicroK8s installed but status unclear" "$status_output"
        MICROK8S_STATUS="WARN"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ microk8s status: WARN"
    fi
}


check_microk8s_addons() {
    log_info "Checking MicroK8s addons..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: microk8s addons (gpu, dns, hostpath-storage)"

    # Get microk8s status once
    local status_output
    if ! status_output=$(sudo microk8s status 2>&1); then
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ Failed to get microk8s status"
        record_result "addon_check" "FAIL" "Failed to get MicroK8s status"
        return 1
    fi

    # Extract enabled addons section
    local enabled_section
    enabled_section=$(echo "$status_output" | awk '
        /enabled:/ {flag=1; next}
        /disabled:/ {flag=0}
        /^[^ ]/ {if(flag && $0 !~ /^  /) flag=0}
        flag && /^  / {print}
    ')

    # Check GPU addon
    if echo "$enabled_section" | grep -qiE "^\s*(gpu|nvidia)(\s|$)"; then
        record_result "addon_gpu" "PASS" "GPU addon is enabled"
    else
        record_result "addon_gpu" "FAIL" "GPU addon is not enabled" "$enabled_section"
    fi

    # Check DNS addon
    if echo "$enabled_section" | grep -qiE "^\s*dns(\s|$)"; then
        record_result "addon_dns" "PASS" "DNS addon is enabled"
    else
        record_result "addon_dns" "FAIL" "DNS addon is not enabled" "$enabled_section"
    fi

    # Check Hostpath-storage addon
    if echo "$enabled_section" | grep -qiE "^\s*(hostpath-storage|storage)(\s|$)"; then
        record_result "addon_hostpath_storage" "PASS" "Hostpath-storage addon is enabled"
    else
        record_result "addon_hostpath_storage" "FAIL" "Hostpath-storage addon is not enabled" "$enabled_section"
    fi
}


check_kubectl_installed() {
    log_info "Checking kubectl installation..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: kubectl version --client"

    # Check if kubectl exists
    if ! command -v kubectl >/dev/null 2>&1; then
        record_result "kubectl_installed" "FAIL" "kubectl not found in PATH"
        KUBECTL_VERSION="N/A"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ kubectl: NOT FOUND"
        return 1
    fi

    # kubectl exists
    local kubectl_path
    kubectl_path=$(command -v kubectl)
    record_result "kubectl_installed" "PASS" "kubectl is installed at $kubectl_path"

    # Check kubectl version
    local version_output
    if version_output=$(kubectl version --client 2>&1); then
        KUBECTL_VERSION=$(echo "$version_output" | grep -oP 'Client Version: v\K[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
        record_result "kubectl_version" "PASS" "kubectl version check successful" "$version_output"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ kubectl version: $KUBECTL_VERSION"
    else
        record_result "kubectl_version" "FAIL" "kubectl version command failed"
        KUBECTL_VERSION="N/A"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ kubectl version: FAILED"
    fi
}


check_helm_installed() {
    log_info "Checking Helm installation..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: helm version"

    # Check if helm exists
    if ! command -v helm >/dev/null 2>&1; then
        record_result "helm_installed" "FAIL" "Helm not found in PATH"
        HELM_VERSION="N/A"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ helm: NOT FOUND"
        return 1
    fi

    # Helm exists
    local helm_path
    helm_path=$(command -v helm)
    record_result "helm_installed" "PASS" "Helm is installed at $helm_path"

    # Check helm version
    local version_output
    if version_output=$(helm version 2>&1); then
        HELM_VERSION=$(echo "$version_output" | grep -oP 'Version:"v\K[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
        record_result "helm_version" "PASS" "Helm version check successful" "$version_output"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ helm version: $HELM_VERSION"
    else
        record_result "helm_version" "FAIL" "Helm version command failed"
        HELM_VERSION="N/A"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ helm version: FAILED"
    fi
}


check_kubeconfig() {
    log_info "Checking kubeconfig file..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: test -f ~/.kube/config"

    local kubeconfig="$HOME/.kube/config"

    if [ ! -f "$kubeconfig" ]; then
        record_result "kubeconfig_exists" "FAIL" "kubeconfig file not found at $kubeconfig"
        KUBECONFIG_EXISTS="false"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ kubeconfig file: NOT FOUND"
        return 1
    fi

    # kubeconfig exists
    record_result "kubeconfig_exists" "PASS" "kubeconfig file exists"
    KUBECONFIG_EXISTS="true"
    echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ kubeconfig file: EXISTS"

    # Check permissions
    local perms
    perms=$(stat -c "%a" "$kubeconfig" 2>/dev/null || stat -f "%OLp" "$kubeconfig" 2>/dev/null || echo "unknown")
    if [[ "$perms" == "600" || "$perms" == "644" ]]; then
        record_result "kubeconfig_permissions" "PASS" "kubeconfig has appropriate permissions ($perms)"
    else
        record_result "kubeconfig_permissions" "WARN" "kubeconfig permissions may be too open ($perms, expected 600)"
    fi

    # Check validity
    if kubectl config view >/dev/null 2>&1; then
        local context
        context=$(kubectl config current-context 2>/dev/null || echo "none")
        record_result "kubeconfig_valid" "PASS" "kubeconfig is valid, current context: $context"
    else
        record_result "kubeconfig_valid" "FAIL" "kubeconfig file is not valid"
    fi
}

check_api_server() {
    log_info "Checking Kubernetes API server..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: kubectl cluster-info"

    # Check if API server is accessible
    local cluster_info
    if cluster_info=$(kubectl cluster-info 2>&1); then
        record_result "api_server_accessible" "PASS" "Kubernetes API server is accessible" "$cluster_info"
        CLUSTER_INFO_STATUS="OK"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ cluster-info: OK"
    else
        record_result "api_server_accessible" "FAIL" "Kubernetes API server is not accessible"
        CLUSTER_INFO_STATUS="ERROR"
        ERROR_MESSAGE="Kubernetes API server not accessible"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ cluster-info: FAILED"
        return 1
    fi

    # Check API server health endpoints
    if kubectl get --raw='/readyz' >/dev/null 2>&1 || kubectl get --raw='/healthz' >/dev/null 2>&1; then
        record_result "api_server_health" "PASS" "API server health check passed"
    else
        record_result "api_server_health" "WARN" "API server health endpoints not accessible"
    fi
}


check_nodes() {
    log_info "Checking Kubernetes nodes..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: kubectl get nodes"

    local nodes_output
    if ! nodes_output=$(kubectl get nodes 2>&1); then
        record_result "nodes_exist" "FAIL" "Failed to query nodes"
        NODES_READY="false"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ nodes: QUERY FAILED"
        return 1
    fi

    local node_count
    node_count=$(echo "$nodes_output" | grep -v "^NAME" | grep -v "^$" | wc -l)

    if [ "$node_count" -gt 0 ]; then
        record_result "nodes_exist" "PASS" "Found $node_count node(s)" "$nodes_output"

        local ready_nodes
        ready_nodes=$(kubectl get nodes -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
        if echo "$ready_nodes" | grep -q "True"; then
            record_result "nodes_ready" "PASS" "At least one node is in Ready state"
            NODES_READY="true"
            echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ nodes: Ready"
        else
            record_result "nodes_ready" "FAIL" "No nodes are in Ready state"
            NODES_READY="false"
            echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ nodes: NOT READY"
        fi
    else
        record_result "nodes_exist" "FAIL" "No nodes found in cluster"
        NODES_READY="false"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ nodes: NONE FOUND"
    fi
}

test_gpu_access() {
    log_info "Testing GPU access with kubectl..."

    # Try running nvidia-smi in a CUDA container using kubectl
    if kubectl run gpu-test --image=nvidia/cuda:11.0.3-base-ubuntu20.04 --rm -i --restart=Never -- nvidia-smi >/dev/null 2>&1; then
        record_result "gpu_access" "PASS" "Successfully ran nvidia-smi, GPU access confirmed"
    else
        record_result "gpu_access" "FAIL" "Could not run nvidia-smi, GPU access failed"
    fi
}

check_system_pods() {
    log_info "Checking system pods..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: kubectl get pods -A"

    local pods_output
    if ! pods_output=$(kubectl get pods -A 2>&1); then
        record_result "pods_exist" "FAIL" "Failed to query pods"
        PODS_ACCESSIBLE="false"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ pods: QUERY FAILED"
        return 1
    fi

    local pod_count
    pod_count=$(echo "$pods_output" | grep -v "^NAMESPACE" | grep -v "^$" | wc -l)

    if [ "$pod_count" -gt 0 ]; then
        record_result "pods_exist" "PASS" "Found $pod_count pod(s) across all namespaces"
        PODS_ACCESSIBLE="true"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ pods: $pod_count found"

        local kube_system_pods
        if kube_system_pods=$(kubectl get pods -n kube-system 2>&1); then
            record_result "kube_system_pods" "PASS" "kube-system namespace pods accessible" "$kube_system_pods"
        else
            record_result "kube_system_pods" "WARN" "Could not query kube-system pods"
        fi
    else
        record_result "pods_exist" "WARN" "No pods found in cluster"
        PODS_ACCESSIBLE="false"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ pods: NONE FOUND"
    fi
}


check_docker_installation() {
    log_info "Checking Docker installation..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: docker info"

    if ! command -v docker >/dev/null 2>&1; then
        record_result "docker_installed" "FAIL" "Docker not found"
        DOCKER_STATUS="NOT_FOUND"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ docker: NOT FOUND"
        return 1
    fi

    record_result "docker_installed" "PASS" "Docker is installed"

    local docker_info
    if ! docker_info=$(docker info 2>&1); then
        record_result "docker_running" "FAIL" "Docker daemon is not running"
        DOCKER_STATUS="ERROR"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ docker: NOT RUNNING"
        return 1
    fi

    record_result "docker_running" "PASS" "Docker daemon is running"
    DOCKER_STATUS="OK"
    echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ docker: Running"

    if echo "$docker_info" | grep -qi "nvidia"; then
        record_result "docker_nvidia_runtime" "PASS" "NVIDIA runtime is configured in Docker"
    else
        record_result "docker_nvidia_runtime" "WARN" "NVIDIA runtime not detected in Docker info"
    fi
}


check_nvidia_driver() {
    log_info "Checking NVIDIA driver..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: nvidia-smi"

    if ! command -v nvidia-smi >/dev/null 2>&1; then
        record_result "nvidia_driver" "FAIL" "nvidia-smi not found"
        NVIDIA_DRIVER_VERSION="NOT_FOUND"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ nvidia-smi: NOT FOUND"
        return 1
    fi

    local nvidia_output
    if ! nvidia_output=$(nvidia-smi 2>&1); then
        record_result "nvidia_driver" "FAIL" "nvidia-smi command failed"
        NVIDIA_DRIVER_VERSION="ERROR"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ nvidia-smi: FAILED"
        return 1
    fi

    local driver_version
    driver_version=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader 2>/dev/null | head -1 || echo "unknown")
    NVIDIA_DRIVER_VERSION="$driver_version"

    local major_version
    major_version=$(echo "$driver_version" | cut -d. -f1)

    if [ "$major_version" -ge 535 ] 2>/dev/null; then
        record_result "nvidia_driver" "PASS" "NVIDIA driver version $driver_version (>= 535)" "$nvidia_output"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ nvidia-smi: $driver_version"
    else
        record_result "nvidia_driver" "WARN" "NVIDIA driver version $driver_version (< 535 recommended)"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ nvidia-smi: $driver_version"
    fi
}

check_nvidia_container_toolkit() {
    log_info "Checking NVIDIA Container Toolkit..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: nvidia-ctk --version"

    if ! command -v nvidia-ctk >/dev/null 2>&1; then
        record_result "nvidia_ctk" "FAIL" "nvidia-ctk not found"
        NVIDIA_CTK_VERSION="NOT_FOUND"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ nvidia-ctk: NOT FOUND"
        return 1
    fi

    local ctk_output
    if ! ctk_output=$(nvidia-ctk --version 2>&1); then
        record_result "nvidia_ctk" "FAIL" "nvidia-ctk command failed"
        NVIDIA_CTK_VERSION="ERROR"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ nvidia-ctk: FAILED"
        return 1
    fi

    local ctk_version
    ctk_version=$(echo "$ctk_output" | grep -oP 'version \K[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
    NVIDIA_CTK_VERSION="$ctk_version"

    if [ "$ctk_version" != "unknown" ]; then
        local major minor
        major=$(echo "$ctk_version" | cut -d. -f1)
        minor=$(echo "$ctk_version" | cut -d. -f2)

        if [ "$major" -gt 1 ] || ([ "$major" -eq 1 ] && [ "$minor" -ge 17 ]) 2>/dev/null; then
            record_result "nvidia_ctk" "PASS" "NVIDIA CTK version $ctk_version (>= 1.17)" "$ctk_output"
            echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ nvidia-ctk: $ctk_version"
        else
            record_result "nvidia_ctk" "WARN" "NVIDIA CTK version $ctk_version (< 1.17 recommended)"
            echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ nvidia-ctk: $ctk_version"
        fi
    else
        record_result "nvidia_ctk" "PASS" "NVIDIA CTK is installed" "$ctk_output"
    fi
}


check_cdi_configuration() {
    log_info "Checking CDI configuration..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: nvidia-ctk cdi list"

    if ! command -v nvidia-ctk >/dev/null 2>&1; then
        record_result "cdi_devices" "WARN" "nvidia-ctk not available for CDI check"
        return 1
    fi

    local cdi_output
    if ! cdi_output=$(nvidia-ctk cdi list 2>&1); then
        record_result "cdi_devices" "WARN" "CDI list command failed"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ CDI: List failed"
        return 1
    fi

    local device_count
    device_count=$(echo "$cdi_output" | grep -c "nvidia.com/gpu" || echo "0")

    if [ "$device_count" -gt 0 ]; then
        record_result "cdi_devices" "PASS" "CDI enabled, $device_count device(s) found" "$cdi_output"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ CDI: $device_count device(s)"
    else
        record_result "cdi_devices" "WARN" "CDI configured but no devices found" "$cdi_output"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ CDI: No devices"
    fi
}

check_user_docker_permissions() {
    log_info "Checking user Docker permissions..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: User in docker group"

    local current_user
    current_user=$(whoami)

    if id -nG "$current_user" 2>/dev/null | grep -qw docker; then
        record_result "user_docker_group" "PASS" "User $current_user is in docker group"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ User docker group: Yes"
    else
        record_result "user_docker_group" "WARN" "User $current_user is not in docker group (may need re-login)"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ User docker group: No"
    fi
}


check_storage_class() {
    log_info "Checking storage class..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: kubectl get storageclass"

    local sc_output
    if ! sc_output=$(kubectl get storageclass 2>&1); then
        record_result "storage_class" "FAIL" "Failed to query storage classes"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ Storage class: QUERY FAILED"
        return 1
    fi

    if echo "$sc_output" | grep -q "microk8s-hostpath"; then
        if echo "$sc_output" | grep "microk8s-hostpath" | grep -q "(default)"; then
            record_result "storage_class" "PASS" "microk8s-hostpath is the default storage class" "$sc_output"
            echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ Storage class: microk8s-hostpath (default)"
        else
            record_result "storage_class" "WARN" "microk8s-hostpath exists but not set as default" "$sc_output"
            echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ Storage class: Not default"
        fi
    else
        record_result "storage_class" "FAIL" "microk8s-hostpath storage class not found" "$sc_output"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ Storage class: Not found"
    fi
}


check_gpu_node_labels() {
    log_info "Checking GPU node labels..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: Node GPU labels"

    local node_labels
    if ! node_labels=$(kubectl get nodes --show-labels 2>&1); then
        record_result "gpu_node_labels" "FAIL" "Failed to query node labels"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ GPU labels: QUERY FAILED"
        return 1
    fi

    if echo "$node_labels" | grep -qi "nvidia"; then
        record_result "gpu_node_labels" "PASS" "GPU labels found on nodes" "$node_labels"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ GPU labels: Found"
    else
        record_result "gpu_node_labels" "WARN" "No GPU labels found on nodes" "$node_labels"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ GPU labels: Not found"
    fi
}


check_ephemeral_storage() {
    log_info "Checking ephemeral storage..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: /ephemeral directory"

    if [ -d /ephemeral ]; then
        record_result "ephemeral_storage" "PASS" "/ephemeral directory exists"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ Ephemeral storage: EXISTS"
    else
        record_result "ephemeral_storage" "WARN" "/ephemeral directory not found"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ Ephemeral storage: NOT FOUND"
    fi
}


check_system_services() {
    log_info "Checking system services..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: Critical system services"

    # Check Docker service
    if systemctl is-active docker >/dev/null 2>&1; then
        record_result "service_docker" "PASS" "Docker service is active"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ Service docker: active"
    else
        record_result "service_docker" "FAIL" "Docker service is not active"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ Service docker: inactive"
    fi

    # Check MicroK8s kubelite service
    if systemctl is-active snap.microk8s.daemon-kubelite >/dev/null 2>&1; then
        record_result "service_microk8s" "PASS" "MicroK8s kubelite service is active"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ Service microk8s: active"
    else
        record_result "service_microk8s" "WARN" "MicroK8s kubelite service is not active"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ Service microk8s: inactive"
    fi
}


check_gpu_kubernetes_detection() {
    log_info "Checking GPU detection in Kubernetes..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: GPU resources in nodes"

    local node_desc
    if ! node_desc=$(kubectl describe node 2>&1); then
        record_result "gpu_k8s_detection" "FAIL" "Failed to describe nodes"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ K8s GPU detection: FAILED"
        return 1
    fi

    local gpu_capacity
    gpu_capacity=$(echo "$node_desc" | grep -A 10 "Capacity:" | grep "nvidia.com/gpu" || true)

    if [ -n "$gpu_capacity" ]; then
        local gpu_count
        gpu_count=$(echo "$gpu_capacity" | awk '{print $2}' || echo "unknown")
        record_result "gpu_k8s_detection" "PASS" "GPU detected in Kubernetes (nvidia.com/gpu: $gpu_count)" "$gpu_capacity"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ K8s GPU detection: $gpu_count GPU(s)"
    else
        record_result "gpu_k8s_detection" "WARN" "No GPU resources detected in node capacity" "$node_desc"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ K8s GPU detection: None"
    fi
}


check_network_connectivity() {
    log_info "Checking network connectivity..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: Cluster DNS resolution"
    
    local test_pod_name="dns-test-$$"
    
    if kubectl run "$test_pod_name" --image=busybox:1.28 --rm -i --restart=Never --command -- nslookup kubernetes.default >/dev/null 2>&1; then
        record_result "network_dns" "PASS" "Cluster DNS resolution working"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ Network DNS: Working"
    else
        if timeout 10 kubectl run "$test_pod_name" --image=busybox:1.28 --rm -i --restart=Never --command -- nslookup kubernetes.default >/dev/null 2>&1; then
            record_result "network_dns" "PASS" "Cluster DNS resolution working (slow)"
            echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ Network DNS: Working (slow)"
        else
            record_result "network_dns" "WARN" "Cluster DNS test failed or timed out"
            echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ Network DNS: Failed"
            # Cleanup test pod if it exists
            kubectl delete pod "$test_pod_name" --ignore-not-found=true >/dev/null 2>&1 || true
        fi
    fi
}


check_dashboard_repo() {
    log_info "Checking Kubernetes Dashboard Helm repository..."

    if helm repo list 2>/dev/null | grep -q "kubernetes-dashboard"; then
        record_result "dashboard_repo" "PASS" "Dashboard Helm repository is configured"
    else
        record_result "dashboard_repo" "WARN" "Dashboard Helm repository not found (may not be installed)"
    fi
}


check_dashboard_deployment() {
    log_info "Checking Kubernetes Dashboard deployment..."

    local deployments
    if deployments=$(kubectl get deployments -n kubernetes-dashboard 2>&1); then
        if echo "$deployments" | grep -q "kubernetes-dashboard"; then
            record_result "dashboard_deployment" "PASS" "Dashboard deployment exists" "$deployments"
        else
            record_result "dashboard_deployment" "WARN" "Dashboard deployment not found" "$deployments"
        fi
    else
        record_result "dashboard_deployment" "WARN" "kubernetes-dashboard namespace not found"
    fi
}


check_dashboard_pods() {
    log_info "Checking Kubernetes Dashboard pods..."

    local pods
    if pods=$(kubectl get pods -n kubernetes-dashboard 2>&1); then
        local running_pods
        running_pods=$(echo "$pods" | grep -c "Running" || echo "0")

        if [ "$running_pods" -gt 0 ]; then
            record_result "dashboard_pods" "PASS" "Found $running_pods running dashboard pod(s)" "$pods"
        else
            record_result "dashboard_pods" "WARN" "No running dashboard pods found" "$pods"
        fi
    else
        record_result "dashboard_pods" "WARN" "Could not query dashboard pods"
    fi
}


check_dashboard_proxy() {
    log_info "Checking Dashboard edge proxy..."

    local daemonset
    if daemonset=$(kubectl get daemonset -n kubernetes-dashboard kdash-edge 2>&1); then
        record_result "dashboard_proxy_daemonset" "PASS" "Dashboard edge proxy DaemonSet exists" "$daemonset"

        local proxy_pods
        if proxy_pods=$(kubectl get pods -n kubernetes-dashboard -l app=kdash-edge 2>&1); then
            if echo "$proxy_pods" | grep -q "Running"; then
                record_result "dashboard_proxy_pods" "PASS" "Dashboard proxy pods are running" "$proxy_pods"
            else
                record_result "dashboard_proxy_pods" "WARN" "Dashboard proxy pods not running" "$proxy_pods"
            fi
        fi
    else
        record_result "dashboard_proxy_daemonset" "WARN" "Dashboard edge proxy DaemonSet not found"
    fi
}


check_dashboard_access() {
    log_info "Checking Dashboard accessibility..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: curl -s -o /dev/null -w \"%{http_code}\" http://localhost:8001/"

    local http_code
    http_code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8001/ 2>/dev/null || echo "000")

    if [[ "$http_code" =~ ^(200|301|302)$ ]]; then
        record_result "dashboard_accessible" "PASS" "Dashboard is accessible on port 8001 (HTTP $http_code)"
        DASHBOARD_ACCESSIBLE="true"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ dashboard: Accessible (HTTP $http_code)"
    else
        record_result "dashboard_accessible" "FAIL" "Dashboard not accessible on port 8001 (HTTP $http_code)"
        DASHBOARD_ACCESSIBLE="false"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✗ dashboard: NOT ACCESSIBLE (HTTP $http_code)"
        ERROR_MESSAGE="${ERROR_MESSAGE:-Dashboard not accessible on port 8001}"
    fi
}


check_dashboard_token() {
    log_info "Checking Dashboard token file..."
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Checking: sudo test -f /root/dashboard-admin-token.txt"
    
    # Check if token file exists
    local token_check=$(sudo test -f /root/dashboard-admin-token.txt && echo "exists" || echo "missing" 2>/dev/null)
    
    if [ "$token_check" = "exists" ]; then
        record_result "dashboard_token_exists" "PASS" "Dashboard token file exists"
        DASHBOARD_TOKEN_EXISTS="true"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ dashboard token: EXISTS"
        
        # Check permissions
        local perms=$(sudo stat -c "%a" /root/dashboard-admin-token.txt 2>/dev/null || sudo stat -f "%OLp" /root/dashboard-admin-token.txt 2>/dev/null || echo "unknown")
        if [ "$perms" = "600" ]; then
            record_result "dashboard_token_permissions" "PASS" "Token file has correct permissions ($perms)"
        else
            record_result "dashboard_token_permissions" "WARN" "Token file permissions may be incorrect ($perms, expected 600)"
        fi
        
        # Check if token file is non-empty
        local token_size=$(sudo cat /root/dashboard-admin-token.txt 2>/dev/null | wc -c || echo "0")
        if [ "$token_size" -gt 0 ]; then
            record_result "dashboard_token_valid" "PASS" "Token file is non-empty ($token_size bytes)"
        else
            record_result "dashboard_token_valid" "FAIL" "Token file is empty or cannot be read"
        fi
    else
        record_result "dashboard_token_exists" "WARN" "Dashboard token file not found (dashboard may not be enabled)"
        DASHBOARD_TOKEN_EXISTS="false"
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ dashboard token: NOT FOUND"
    fi
}


check_user_permissions() {
    log_info "Checking user permissions..."

    # Check if user can run kubectl commands
    if kubectl get nodes >/dev/null 2>&1; then
        record_result "user_kubectl_access" "PASS" "User can run kubectl commands"
    else
        record_result "user_kubectl_access" "FAIL" "User cannot run kubectl commands"
    fi

    # Check if user is in microk8s group
    if groups | grep -qw microk8s; then
        record_result "user_microk8s_group" "PASS" "User is in microk8s group"
    else
        record_result "user_microk8s_group" "WARN" "User not in microk8s group (may need logout/login)"
    fi
}

output_csv() {
    local overall_status="$1"
    local duration="$2"
    
    MICROK8S_STATUS="${MICROK8S_STATUS:-N/A}"
    KUBECTL_VERSION="${KUBECTL_VERSION:-N/A}"
    HELM_VERSION="${HELM_VERSION:-N/A}"
    CLUSTER_INFO_STATUS="${CLUSTER_INFO_STATUS:-N/A}"
    NODES_READY="${NODES_READY:-false}"
    PODS_ACCESSIBLE="${PODS_ACCESSIBLE:-false}"
    KUBECONFIG_EXISTS="${KUBECONFIG_EXISTS:-false}"
    DASHBOARD_ACCESSIBLE="${DASHBOARD_ACCESSIBLE:-false}"
    DASHBOARD_TOKEN_EXISTS="${DASHBOARD_TOKEN_EXISTS:-false}"
    DOCKER_STATUS="${DOCKER_STATUS:-N/A}"
    NVIDIA_DRIVER_VERSION="${NVIDIA_DRIVER_VERSION:-N/A}"
    NVIDIA_CTK_VERSION="${NVIDIA_CTK_VERSION:-N/A}"
    GPU_OPERATOR_STATUS="${GPU_OPERATOR_STATUS:-N/A}"
    ERROR_MESSAGE="${ERROR_MESSAGE:-}"
    
    
    if [ ! -f "$CSV_FILE" ]; then
        echo "overall_status,microk8s_status,kubectl_version,helm_version,cluster_info,nodes_ready,pods_accessible,kubeconfig_exists,dashboard_accessible,dashboard_token_exists,docker_status,nvidia_driver_version,nvidia_ctk_version,gpu_operator_status,build_duration_seconds,error_message,environment_id,timestamp" > "$CSV_FILE"
    fi
    
    echo "$overall_status,$MICROK8S_STATUS,$KUBECTL_VERSION,$HELM_VERSION,$CLUSTER_INFO_STATUS,$NODES_READY,$PODS_ACCESSIBLE,$KUBECONFIG_EXISTS,$DASHBOARD_ACCESSIBLE,$DASHBOARD_TOKEN_EXISTS,$DOCKER_STATUS,$NVIDIA_DRIVER_VERSION,$NVIDIA_CTK_VERSION,$GPU_OPERATOR_STATUS,$duration,$ERROR_MESSAGE,$hostname,$TIMESTAMP" >> "$CSV_FILE"
}

print_summary() {
    local overall_status="$1"
    local duration="$2"
    
    wait
    log_plain ""
    log_plain "=== Kubernetes Installation Validation Summary ==="
    log_plain "Total checks: $TOTAL_CHECKS"
    
    local success_pct=0
    local partial_pct=0
    local failed_pct=0
    
    if [ "$TOTAL_CHECKS" -gt 0 ]; then
        success_pct=$(awk "BEGIN {printf \"%.1f\", ($PASSED_CHECKS/$TOTAL_CHECKS)*100}")
        partial_pct=$(awk "BEGIN {printf \"%.1f\", ($WARNINGS/$TOTAL_CHECKS)*100}")
        failed_pct=$(awk "BEGIN {printf \"%.1f\", ($FAILED_CHECKS/$TOTAL_CHECKS)*100}")
    fi
    
    log_plain "Successful: $PASSED_CHECKS (${success_pct}%)"
    log_plain "Partial: $WARNINGS (${partial_pct}%)"
    log_plain "Failed: $FAILED_CHECKS (${failed_pct}%)"
    
    if [ -n "$ERROR_MESSAGE" ]; then
        log_plain ""
        log_plain "Error Details:"
        log_plain "- $ERROR_MESSAGE"
    fi
    
    log_plain ""
    log_plain "Log file saved to: $LOG_FILE"
    log_plain "CSV results saved to: $CSV_FILE"
    log_plain ""
}

main() {
    _log_to_file "INFO" "Kubernetes Installation Validation Script Started"
    _log_to_file "INFO" "Timestamp: $TIMESTAMP"
    _log_to_file "INFO" "Log file: $LOG_FILE"
    
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] Starting K8s validation"
    log_plain "======================================================================"
    log_plain "Kubernetes Installation Validation Script"
    log_plain "======================================================================"
    log_plain ""
    
    # Add /snap/bin to PATH to ensure snap-installed binaries are found
    export PATH="/snap/bin:$PATH"
    # ============================================
    # 1. MICROK8S INFRASTRUCTURE
    # ============================================
    check_microk8s_installed
    check_microk8s_running
    check_microk8s_addons
    log_plain ""

    # ============================================
    # 2. CLIENT TOOLS
    # ============================================
    check_kubectl_installed
    check_helm_installed
    log_plain ""

    # ============================================
    # 3. CLUSTER CONFIGURATION
    # ============================================
    check_kubeconfig
    log_plain ""

    # ============================================
    # 4. CLUSTER CONNECTIVITY & STATUS
    # ============================================
    check_api_server
    check_network_connectivity
    check_nodes
    test_gpu_access
    check_system_pods
    log_plain ""

    # ============================================
    # 5. USER PERMISSIONS
    # ============================================
    check_user_permissions
    log_plain ""

    # ============================================
    # 6. CONTAINER RUNTIME
    # ============================================
    check_docker_installation
    check_user_docker_permissions
    log_plain ""

    # ============================================
    # 7. GPU INFRASTRUCTURE
    # ============================================
    check_nvidia_driver
    check_nvidia_container_toolkit
    check_cdi_configuration
    check_gpu_node_labels
    check_gpu_kubernetes_detection
    log_plain ""

    # ============================================
    # 8. STORAGE & SYSTEM SERVICES
    # ============================================
    check_storage_class
    check_ephemeral_storage
    check_system_services
    log_plain ""

    # ============================================
    # 9. DASHBOARD COMPONENTS
    # ============================================
    check_dashboard_repo
    check_dashboard_deployment
    check_dashboard_pods
    check_dashboard_proxy
    check_dashboard_access
    check_dashboard_token    
    log_plain ""
    log_plain ""
    
    if [ "$FAILED_CHECKS" -eq 0 ]; then
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ✓ All checks passed"
    else
        echo "[$(date +"%Y-%m-%d %H:%M:%S")]   ⚠ Some checks failed or have warnings"
    fi
    
    local script_end_time=$(date +%s)
    local duration=$((script_end_time - SCRIPT_START_TIME))
    
    local exit_code=0
    local overall_status=""
    if [ "$FAILED_CHECKS" -eq 0 ]; then
        overall_status="SUCCESS"
        exit_code=0
    elif [ "$MICROK8S_STATUS" = "ERROR" ] || [ -z "$KUBECTL_VERSION" ] || [ "$KUBECTL_VERSION" = "N/A" ]; then
        overall_status="FAILED"
        exit_code=2
    elif [ "$FAILED_CHECKS" -lt "$TOTAL_CHECKS" ]; then
        overall_status="PARTIAL"
        exit_code=1
    else
        overall_status="FAILED"
        exit_code=2
    fi
    
    _log_to_file "SUMMARY" "Overall Status: $overall_status"
    
    output_csv "$overall_status" "$duration"
    print_summary "$overall_status" "$duration"
    
    exit $exit_code
}

main "$@"
