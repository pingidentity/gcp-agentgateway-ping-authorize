BAATT := baseline-autonomous-agent-to-tool/services/agent
AOBOU := agent-on-behalf-of-user/services/agent

# All Agent Runtime Reasoning Engines in the same project and region must use
# the same Agent-to-Anywhere gateway for egress. Multiple engines can share
# that gateway, but different engines cannot select different egress gateways.
# Allow a few minutes for Agent Runtime and iap.egressor IAM propagation after
# deploying or changing a gateway-bound engine.

.PHONY: switch-to-baatt switch-to-aobou

switch-to-baatt:
	@echo "==> Tearing down aobou agent..."
	$(MAKE) -C $(AOBOU) teardown
	@echo "==> Deploying baatt agent..."
	$(MAKE) -C $(BAATT) deploy

switch-to-aobou:
	@echo "==> Tearing down baatt agent..."
	$(MAKE) -C $(BAATT) teardown
	@echo "==> Deploying aobou agent..."
	$(MAKE) -C $(AOBOU) deploy
