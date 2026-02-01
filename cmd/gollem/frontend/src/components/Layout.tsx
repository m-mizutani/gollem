import type { ReactNode } from "react";
import { Link } from "react-router-dom";

interface LayoutProps {
  children: ReactNode;
}

export default function Layout({ children }: LayoutProps) {
  return (
    <div className="min-h-screen flex flex-col">
      <header className="bg-white border-b border-gray-200 px-6 py-3 flex items-center justify-between">
        <Link to="/" className="text-lg font-semibold text-gray-800 hover:text-gray-600">
          gollem trace viewer
        </Link>
        <Link to="/license" className="text-xs text-gray-400 hover:text-gray-600">
          Licenses
        </Link>
      </header>
      <main className="flex-1 p-6">{children}</main>
    </div>
  );
}
