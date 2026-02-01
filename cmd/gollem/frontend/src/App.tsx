import { Routes, Route } from "react-router-dom";
import Layout from "./components/Layout";
import TraceListPage from "./components/TraceListPage";
import TraceDetailPage from "./components/TraceDetailPage";
import LicensePage from "./components/LicensePage";

function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<TraceListPage />} />
        <Route path="/traces/:id" element={<TraceDetailPage />} />
        <Route path="/license" element={<LicensePage />} />
      </Routes>
    </Layout>
  );
}

export default App;
