import { useState } from "react";
import { useNavigate } from "react-router-dom";

export default function TraceIDInput() {
  const [value, setValue] = useState("");
  const navigate = useNavigate();

  const handleSubmit = () => {
    const trimmed = value.trim();
    if (trimmed) {
      navigate(`/traces/${trimmed}`);
    }
  };

  return (
    <div className="flex gap-2">
      <input
        type="text"
        placeholder="Enter Trace ID..."
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") handleSubmit();
        }}
        className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
      />
      <button
        onClick={handleSubmit}
        disabled={!value.trim()}
        className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        Go
      </button>
    </div>
  );
}
