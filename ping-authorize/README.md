# PingAuthorize on GCE VM (PoC)

These steps show how to run **PingAuthorize Server** and **PingAuthorize Policy Editor (PAP)** on a **GCE VM** and expose the JSON PDP endpoint at `/governance-engine` for PoC use.

> Replace all placeholder values (e.g. `<VM_EXTERNAL_IP>`).


## 1. Create a GCE VM

In the GCP Console:

1. Go to **Compute Engine → VM instances → Create instance**.
2. Configure:
   - **Name**: your choice (e.g. `pingauthorize-vm`)
   - **Region / Zone**: where you want to run (e.g. `us-central1 / us-central1-a`)
   - **Machine type**: `e2-standard-2` (or similar)
   - **Boot disk**: **Ubuntu 22.04 LTS**
   - **Network tags**: `pingauthorize`
3. Click **Create**.

## 2. Install Docker on the VM

SSH into the VM from the console:

```bash
sudo apt-get update
sudo apt-get install -y docker.io
sudo usermod -aG docker $USER
exit
```

Re-SSH into the VM so Docker group membership takes effect.

## 3. Add Ping DevOps credentials

On the VM:

```bash
mkdir -p ~/.pingidentity

cat > ~/.pingidentity/config << 'EOF'
PING_IDENTITY_DEVOPS_USER=<YOUR_DEVOPS_USERNAME_HERE>
PING_IDENTITY_DEVOPS_KEY=<YOUR_DEVOPS_KEY_HERE>
PING_IDENTITY_ACCEPT_EULA=YES
EOF
```

## 4. Run PingAuthorize Server + PAP (Docker)

### 4.1 PingAuthorize Server

```bash
docker run \
  --name pingauthorize \
  --env-file ~/.pingidentity/config \
  --publish 7443:1443 \
  --detach \
  --env SERVER_PROFILE_URL=https://github.com/pingidentity/pingidentity-server-profiles.git \
  --env SERVER_PROFILE_PATH=getting-started/pingauthorize \
  pingidentity/pingauthorize:edge
```

### 4.2 PingAuthorize Policy Editor (PAP)

1. From the VM instances page, note the VM’s **External IP** as `<VM_EXTERNAL_IP>`.

```bash
docker run \
  --name pingauthorizepap \
  --env-file ~/.pingidentity/config \
  --env PING_EXTERNAL_BASE_URL=<VM_EXTERNAL_IP>:8443 \
  --publish 8443:1443 \
  --detach \
  pingidentity/pingauthorizepap:edge
```

Verify both containers:

```bash
docker container ls
```

## 5. Open firewall ports

In GCP Console → **VPC network → Firewall → Create firewall rule**:

1. **PingAuthorize (7443)**  
   - Name: `allow-pingauthorize-7443`  
   - Targets: **Specified target tags** → `pingauthorize`  
   - Source ranges: `0.0.0.0/0` (tighten as needed)  
   - Protocols/ports: `tcp:7443`

2. **PAP GUI (8443)**  
   - Name: `allow-pingauthorize-pap-8443`  
   - Targets: `pingauthorize`  
   - Source ranges: your IP (or `0.0.0.0/0` for PoC)  
   - Protocols/ports: `tcp:8443`

## 6. Configure PAP (Policy Editor)

### 6.1 Access PAP

```text
https://<VM_EXTERNAL_IP>:8443
```

Default DevOps demo credentials:

- Username: `admin`
- Password: `password123`

### 6.2 Create a branch

In the PAP UI, go to **Branch Manager** and create a branch, for example:
```text
shim-poc
```

### 6.3 Create attribute

- Name: `X-PA-Decision`
- Type: `String`
- Add a **Request** resolver

### 6.4 Create a simple policy

**Rule 1 – Permit**
- Condition: `X-PA-Decision == "permit"`

**Rule 2 – Deny**
- Condition: `X-PA-Decision != "permit"`

### 6.5 Get the policy ID

Use this later as `<POLICY_ID>`.

## 7. Wire PingAuthorize Server to PAP (external PDP)

### 7.1 Get PAP container IP

```bash
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' pingauthorizepap
```

### 7.2 Create a Policy External Server for PAP

```bash
docker exec pingauthorize \
  /opt/out/instance/bin/dsconfig \
  create-external-server \
  --server-name "PAP" \
  --type policy \
  --set "base-url:https://<PAP_CONTAINER_IP>:1443" \
  --set "shared-secret:2FederateM0re" \
  --set "branch:shim-poc" \
  --no-prompt
```

### 7.3 Enable Blind Trust (PoC TLS)

```bash
docker exec pingauthorize \
  /opt/out/instance/bin/dsconfig \
  set-trust-manager-provider-prop \
  --provider-name "Blind Trust" \
  --set enabled:true \
  --no-prompt
```

```bash
docker exec pingauthorize \
  /opt/out/instance/bin/dsconfig \
  set-external-server-prop \
  --server-name "PAP" \
  --set "trust-manager-provider:Blind Trust" \
  --no-prompt
```

### 7.4 Set external PDP mode

```bash
docker exec pingauthorize \
  /opt/out/instance/bin/dsconfig \
  set-policy-decision-service-prop \
  --set pdp-mode:external \
  --set "policy-server:PAP" \
  --set trust-framework-version:v2 \
  --no-prompt
```

### 7.5 Set decision-node

```bash
docker exec pingauthorize \
  /opt/out/instance/bin/dsconfig \
  set-external-server-prop \
  --server-name "PAP" \
  --set "decision-node:<POLICY_ID>" \
  --no-prompt
```

## 8. Test the JSON PDP endpoint

```bash
curl -k \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{"attributes": {"X-PA-Decision": "permit"}}' \
  https://<VM_EXTERNAL_IP>:7443/governance-engine
```

```bash
curl -k \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{"attributes": {"X-PA-Decision": "deny"}}' \
  https://<VM_EXTERNAL_IP>:7443/governance-engine
```
