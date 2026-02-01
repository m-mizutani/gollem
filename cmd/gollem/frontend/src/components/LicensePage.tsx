import { useEffect, useState } from "react";

export default function LicensePage() {
  const [content, setContent] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch("/licenses.txt")
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.text();
      })
      .then(setContent)
      .catch((e) => setError(e.message));
  }, []);

  if (error) {
    return (
      <div className="max-w-4xl mx-auto">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          Failed to load licenses: {error}
        </div>
      </div>
    );
  }

  if (content === null) {
    return (
      <div className="max-w-4xl mx-auto text-gray-500">
        Loading licenses...
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto">
      <h1 className="text-lg font-semibold mb-4">Third-Party Licenses</h1>
      <pre className="bg-gray-50 border border-gray-200 rounded-lg p-4 text-xs overflow-auto whitespace-pre-wrap max-h-[80vh]">
        {content}
      </pre>
    </div>
  );
}
