import { Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { Overview } from "./pages/Overview";
import { VpnPools } from "./pages/VpnPools";
import { VpnPoolDetail } from "./pages/VpnPoolDetail";
import { VpnTunnels } from "./pages/VpnTunnels";
import { VpnTunnelDetail } from "./pages/VpnTunnelDetail";
import { Clients } from "./pages/Clients";
import { ClientDetail } from "./pages/ClientDetail";
import { Adguard } from "./pages/Adguard";
import { Traffic } from "./pages/Traffic";
import { System } from "./pages/System";
import { Firewall } from "./pages/Firewall";
import { Qos } from "./pages/Qos";

function NotFound() {
  return (
    <div className="bg-surface-container p-8 mt-8 text-center">
      <h1 className="text-lg font-semibold mb-4">Not Found</h1>
      <p className="font-mono text-sm text-on-surface-variant">
        No such page.
      </p>
    </div>
  );
}

export function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Overview />} />
        <Route path="/vpn/pools" element={<VpnPools />} />
        <Route path="/vpn/pools/:name" element={<VpnPoolDetail />} />
        <Route path="/vpn/tunnels" element={<VpnTunnels />} />
        <Route path="/vpn/tunnels/:name" element={<VpnTunnelDetail />} />
        <Route path="/clients" element={<Clients />} />
        <Route path="/clients/:ip" element={<ClientDetail />} />
        <Route path="/adguard" element={<Adguard />} />
        <Route path="/traffic" element={<Traffic />} />
        <Route path="/firewall" element={<Firewall />} />
        <Route path="/qos" element={<Qos />} />
        <Route path="/system" element={<System />} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  );
}
