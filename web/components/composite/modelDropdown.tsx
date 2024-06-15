import {DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger} from "@/components/ui/dropdown-menu";
import {ChevronDown} from "lucide-react";
import React from "react";

export default function ModelDropdown() {
    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <button className="flex justify-center gap-2 p-8 w-full">
                    GPT-3.5
                    <ChevronDown/>
                </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent>
                <DropdownMenuItem>
                    GPT-3.5
                </DropdownMenuItem>
                <DropdownMenuItem>
                    GPT-4
                </DropdownMenuItem>
            </DropdownMenuContent>
        </DropdownMenu>
    )
}