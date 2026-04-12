import { Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { Overview } from "./pages/Overview";
import { VpnPools } from "./pages/VpnPools";
import { VpnPoolDetail } from "./pages/VpnPoolDetail";
import { Clients } from "./pages/Clients";
import { ClientDetail } from "./pages/ClientDetail";
import { Adguard } from "./pages/Adguard";
import { Placeholder } from "./pages/Placeholder";

export function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Overview />} />
        <Route path="/vpn/pools" element={<VpnPools />} />
        <Route path="/vpn/pools/:name" element={<VpnPoolDetail />} />
        <Route
          path="/vpn/tunnels"
          element={<Placeholder title="VPN Tunnels" />}
        />
        <Route
          path="/vpn/tunnels/:name"
          element={<Placeholder title="VPN Tunnel Detail" />}
        />
        <Route path="/clients" element={<Clients />} />
        <Route path="/clients/:ip" element={<ClientDetail />} />
        <Route path="/adguard" element={<Adguard />} />
        <Route path="/traffic" element={<Placeholder title="Traffic" />} />
        <Route
          path="/firewall"
          element={<Placeholder title="Firewall & PBR" />}
        />
        <Route path="/qos" element={<Placeholder title="QoS" />} />
        <Route path="/system" element={<Placeholder title="System" />} />
        <Route path="*" element={<Placeholder title="Not Found" />} />
      </Route>
    </Routes>
  );
}
