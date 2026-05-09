import { Link } from "react-router-dom";

interface BreadcrumbProps {
  path: string;
}

export default function Breadcrumb({ path }: BreadcrumbProps) {
  const segments = path.split("/").filter((s) => s.length > 0);

  const linkFor = (depth: number): string => {
    const sub = segments.slice(0, depth).join("/");
    return sub === "" ? "/" : `/?path=${encodeURIComponent(sub)}`;
  };

  return (
    <nav className="text-sm text-gray-600">
      <ol className="flex flex-wrap items-center gap-1">
        <li>
          <Link
            to="/"
            className="text-blue-600 hover:text-blue-800 hover:underline"
          >
            root
          </Link>
        </li>
        {segments.map((seg, idx) => {
          const isLast = idx === segments.length - 1;
          return (
            <li key={idx} className="flex items-center gap-1">
              <span className="text-gray-400">/</span>
              {isLast ? (
                <span className="font-medium text-gray-900">{seg}</span>
              ) : (
                <Link
                  to={linkFor(idx + 1)}
                  className="text-blue-600 hover:text-blue-800 hover:underline"
                >
                  {seg}
                </Link>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
