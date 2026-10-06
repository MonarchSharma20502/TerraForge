// The Azure public icon set, one SVG per resource kind. The catalog's `icon`
// field names the kind, and this module maps it to the asset the bundler
// inlines. Icons keep the template's 1:1 ratio.

import keyVault from "./icons/key_vault.svg";
import linuxVm from "./icons/linux_vm.svg";
import logAnalytics from "./icons/log_analytics.svg";
import nsg from "./icons/nsg.svg";
import privateDnsZone from "./icons/private_dns_zone.svg";
import publicIp from "./icons/public_ip.svg";
import resourceGroup from "./icons/resource_group.svg";
import routeTable from "./icons/route_table.svg";
import storageAccount from "./icons/storage_account.svg";
import subnet from "./icons/subnet.svg";
import vnet from "./icons/vnet.svg";
import windowsVm from "./icons/windows_vm.svg";

// ICONS is the kind -> inlined SVG map. A kind with no asset falls back to the
// kind name as text so the palette is never blank.
export const ICONS: Record<string, string> = {
  key_vault: keyVault,
  linux_vm: linuxVm,
  log_analytics: logAnalytics,
  nsg: nsg,
  private_dns_zone: privateDnsZone,
  public_ip: publicIp,
  resource_group: resourceGroup,
  route_table: routeTable,
  storage_account: storageAccount,
  subnet: subnet,
  vnet: vnet,
  windows_vm: windowsVm,
};

interface KindIconProps {
  kind: string;
  className?: string;
}

// KindIcon renders the Azure SVG for a resource kind. The svg is inlined markup
// rather than an <img> so it scales with the canvas and stays crisp at any zoom.
export function KindIcon({ kind, className }: KindIconProps) {
  const svg = ICONS[kind];
  if (!svg) {
    return <span className={className}>{kind}</span>;
  }
  return (
    <span
      className={className}
      // The asset is a static string produced by the bundler from the SVG file,
      // never user input, so this is the documented way to inline it.
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
