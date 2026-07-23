import { createControlPlaneClient } from "@multi-agent-desk/protocol";

const configuredBase = document.querySelector<HTMLMetaElement>('meta[name="multidesk-control-plane"]')?.content ?? window.location.origin;

export const controlPlane = createControlPlaneClient(configuredBase);
