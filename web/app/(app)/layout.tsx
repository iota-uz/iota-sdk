import {Sidebar} from "@/components/sidebar";
import Navbar from "@/components/navbar";
import React from "react";

export default function Page({children}: { children: React.ReactNode }) {
  return (
    <div key="1" className="grid min-h-screen w-full grid-cols-[280px_1fr]">
      <Sidebar/>
      <div>
        <Navbar/>
        {children}
      </div>
    </div>
  )
}