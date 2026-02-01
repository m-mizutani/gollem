import { useState } from "react";

interface JSONViewProps {
  data: string;
}

export default function JSONView({ data }: JSONViewProps) {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <div className="relative">
      <div className="absolute top-1 right-1 flex gap-1">
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="px-1.5 py-0.5 text-xs text-gray-400 hover:text-gray-600 bg-white rounded"
        >
          {collapsed ? "Expand" : "Collapse"}
        </button>
        <button
          onClick={() => navigator.clipboard.writeText(data)}
          className="px-1.5 py-0.5 text-xs text-gray-400 hover:text-gray-600 bg-white rounded"
        >
          Copy
        </button>
      </div>
      <pre
        className={`bg-gray-900 text-gray-100 rounded p-3 text-xs overflow-auto ${
          collapsed ? "max-h-12" : "max-h-96"
        }`}
      >
        <code>{data}</code>
      </pre>
    </div>
  );
}
