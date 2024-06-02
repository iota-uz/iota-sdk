import Link from "next/link"
import {Toggle} from "@/components/ui/toggle"
import * as Icons from "@phosphor-icons/react"
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger
} from "@/components/ui/dropdown-menu"

export default function Navbar() {
    return (
        <header className="flex h-14 w-full bg-white shadow-md shrink-0 items-center px-4 md:px-6">
            <div className="ml-auto flex items-center gap-4">
                <Toggle aria-label="Toggle theme" className="bg-primary-100 rounded-lg">
                    <Icons.Moon className="h-5 w-5"/>
                </Toggle>
                <DropdownMenu>
                    <DropdownMenuTrigger asChild className="cursor-pointer">
                        <Avatar className="h-10 w-10">
                            <AvatarImage alt="@shadcn" src="/placeholder-avatar.jpg"/>
                            <AvatarFallback className="bg-primary-600 text-white">JP</AvatarFallback>
                        </Avatar>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent>
                        <DropdownMenuLabel>
                            <Link href="/account">
                                Профиль
                            </Link>
                        </DropdownMenuLabel>
                        <DropdownMenuSeparator/>
                        <DropdownMenuItem>
                            <Link href="/settings">Настройки</Link>
                        </DropdownMenuItem>
                        <DropdownMenuSeparator/>
                        <DropdownMenuItem>
                            <Link href="/logout">Выйти</Link>
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            </div>
        </header>
    )
}

