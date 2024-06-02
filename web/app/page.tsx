'use client';
import Link from "next/link"
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger
} from "@/components/ui/dropdown-menu"
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card"
import {JSX, SVGProps} from "react"

export default function Component() {
    return (
        <div className="flex flex-col">
            <header className="flex h-14 items-center justify-between border-b bg-gray-100 px-6 dark:bg-gray-800">
                <div className="flex items-center gap-4">
                    <Link className="lg:hidden" href="#">
                        <MenuIcon className="h-6 w-6"/>
                        <span className="sr-only">Toggle navigation</span>
                    </Link>
                    <h1 className="text-lg font-semibold">Dashboard</h1>
                </div>
                <div className="flex items-center gap-4">
                    <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                            <Avatar className="h-9 w-9">
                                <AvatarImage alt="@shadcn" src="/placeholder-avatar.jpg"/>
                                <AvatarFallback>JP</AvatarFallback>
                            </Avatar>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                            <DropdownMenuItem>Profile</DropdownMenuItem>
                            <DropdownMenuItem>Settings</DropdownMenuItem>
                            <DropdownMenuSeparator/>
                            <DropdownMenuItem>Logout</DropdownMenuItem>
                        </DropdownMenuContent>
                    </DropdownMenu>
                </div>
            </header>
            <main className="flex-1 p-6">
                <div className="grid gap-6">
                    <Card>
                        <CardHeader>
                            <CardTitle>Welcome back, John!</CardTitle>
                            <CardDescription>Here's a quick overview of your dashboard.</CardDescription>
                        </CardHeader>
                        <CardContent>
                            <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
                                <div className="rounded-md bg-gray-100 p-4 dark:bg-gray-800">
                                    <div className="flex items-center justify-between">
                                        <div className="text-sm font-medium">Projects</div>
                                        <BriefcaseIcon className="h-6 w-6 text-gray-500 dark:text-gray-400"/>
                                    </div>
                                    <div className="mt-2 text-2xl font-bold">24</div>
                                </div>
                                <div className="rounded-md bg-gray-100 p-4 dark:bg-gray-800">
                                    <div className="flex items-center justify-between">
                                        <div className="text-sm font-medium">Tasks</div>
                                        <SquareCheckIcon className="h-6 w-6 text-gray-500 dark:text-gray-400"/>
                                    </div>
                                    <div className="mt-2 text-2xl font-bold">142</div>
                                </div>
                                <div className="rounded-md bg-gray-100 p-4 dark:bg-gray-800">
                                    <div className="flex items-center justify-between">
                                        <div className="text-sm font-medium">Clients</div>
                                        <UsersIcon className="h-6 w-6 text-gray-500 dark:text-gray-400"/>
                                    </div>
                                    <div className="mt-2 text-2xl font-bold">18</div>
                                </div>
                                <div className="rounded-md bg-gray-100 p-4 dark:bg-gray-800">
                                    <div className="flex items-center justify-between">
                                        <div className="text-sm font-medium">Revenue</div>
                                        <CurrencyIcon className="h-6 w-6 text-gray-500 dark:text-gray-400"/>
                                    </div>
                                    <div className="mt-2 text-2xl font-bold">$42,000</div>
                                </div>
                            </div>
                        </CardContent>
                    </Card>
                    <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
                        <Card>
                            <CardHeader>
                                <CardTitle>Recent Projects</CardTitle>
                                <CardDescription>A summary of your recent project activity.</CardDescription>
                            </CardHeader>
                            <CardContent>
                                <div className="space-y-4">
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center gap-3">
                                            <Avatar className="h-10 w-10">
                                                <AvatarImage alt="Project logo" src="/placeholder-logo.svg"/>
                                                <AvatarFallback>P</AvatarFallback>
                                            </Avatar>
                                            <div>
                                                <div className="font-medium">Acme Website</div>
                                                <div className="text-sm text-gray-500 dark:text-gray-400">Redesign
                                                    and development
                                                </div>
                                            </div>
                                        </div>
                                        <div className="text-sm font-medium text-gray-500 dark:text-gray-400">2 days
                                            ago
                                        </div>
                                    </div>
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center gap-3">
                                            <Avatar className="h-10 w-10">
                                                <AvatarImage alt="Project logo" src="/placeholder-logo.svg"/>
                                                <AvatarFallback>P</AvatarFallback>
                                            </Avatar>
                                            <div>
                                                <div className="font-medium">Marketing Dashboard</div>
                                                <div className="text-sm text-gray-500 dark:text-gray-400">Analytics
                                                    and reporting
                                                </div>
                                            </div>
                                        </div>
                                        <div className="text-sm font-medium text-gray-500 dark:text-gray-400">1 week
                                            ago
                                        </div>
                                    </div>
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center gap-3">
                                            <Avatar className="h-10 w-10">
                                                <AvatarImage alt="Project logo" src="/placeholder-logo.svg"/>
                                                <AvatarFallback>P</AvatarFallback>
                                            </Avatar>
                                            <div>
                                                <div className="font-medium">Mobile App Redesign</div>
                                                <div className="text-sm text-gray-500 dark:text-gray-400">UX and UI
                                                    updates
                                                </div>
                                            </div>
                                        </div>
                                        <div className="text-sm font-medium text-gray-500 dark:text-gray-400">2
                                            weeks ago
                                        </div>
                                    </div>
                                </div>
                            </CardContent>
                        </Card>
                        <Card>
                            <CardHeader>
                                <CardTitle>Team Activity</CardTitle>
                                <CardDescription>A summary of your team's recent activity.</CardDescription>
                            </CardHeader>
                            <CardContent>
                                <div className="space-y-4">
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center gap-3">
                                            <Avatar className="h-10 w-10">
                                                <AvatarImage alt="User avatar" src="/placeholder-avatar.jpg"/>
                                                <AvatarFallback>JD</AvatarFallback>
                                            </Avatar>
                                            <div>
                                                <div className="font-medium">John Doe</div>
                                                <div className="text-sm text-gray-500 dark:text-gray-400">
                                                    Completed task: Wireframe design
                                                </div>
                                            </div>
                                        </div>
                                        <div className="text-sm font-medium text-gray-500 dark:text-gray-400">2 days
                                            ago
                                        </div>
                                    </div>
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center gap-3">
                                            <Avatar className="h-10 w-10">
                                                <AvatarImage alt="User avatar" src="/placeholder-avatar.jpg"/>
                                                <AvatarFallback>JD</AvatarFallback>
                                            </Avatar>
                                            <div>
                                                <div className="font-medium">Jane Doe</div>
                                                <div className="text-sm text-gray-500 dark:text-gray-400">Commented
                                                    on a project
                                                </div>
                                            </div>
                                        </div>
                                        <div className="text-sm font-medium text-gray-500 dark:text-gray-400">1 week
                                            ago
                                        </div>
                                    </div>
                                    <div className="flex items-center justify-between">
                                        <div className="flex items-center gap-3">
                                            <Avatar className="h-10 w-10">
                                                <AvatarImage alt="User avatar" src="/placeholder-avatar.jpg"/>
                                                <AvatarFallback>JD</AvatarFallback>
                                            </Avatar>
                                            <div>
                                                <div className="font-medium">John Smith</div>
                                                <div className="text-sm text-gray-500 dark:text-gray-400">Joined the
                                                    team
                                                </div>
                                            </div>
                                        </div>
                                        <div className="text-sm font-medium text-gray-500 dark:text-gray-400">2
                                            weeks ago
                                        </div>
                                    </div>
                                </div>
                            </CardContent>
                        </Card>
                    </div>
                </div>
            </main>
        </div>
    )
}

function BriefcaseIcon(props: JSX.IntrinsicAttributes & SVGProps<SVGSVGElement>) {
    return (
        <svg
            {...props}
            xmlns="http://www.w3.org/2000/svg"
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
        >
            <path d="M16 20V4a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"/>
            <rect width="20" height="14" x="2" y="6" rx="2"/>
        </svg>
    )
}


function CurrencyIcon(props: JSX.IntrinsicAttributes & SVGProps<SVGSVGElement>) {
    return (
        <svg
            {...props}
            xmlns="http://www.w3.org/2000/svg"
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
        >
            <circle cx="12" cy="12" r="8"/>
            <line x1="3" x2="6" y1="3" y2="6"/>
            <line x1="21" x2="18" y1="3" y2="6"/>
            <line x1="3" x2="6" y1="21" y2="18"/>
            <line x1="21" x2="18" y1="21" y2="18"/>
        </svg>
    )
}


function MenuIcon(props: JSX.IntrinsicAttributes & SVGProps<SVGSVGElement>) {
    return (
        <svg
            {...props}
            xmlns="http://www.w3.org/2000/svg"
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
        >
            <line x1="4" x2="20" y1="12" y2="12"/>
            <line x1="4" x2="20" y1="6" y2="6"/>
            <line x1="4" x2="20" y1="18" y2="18"/>
        </svg>
    )
}

function SquareCheckIcon(props: JSX.IntrinsicAttributes & SVGProps<SVGSVGElement>) {
    return (
        <svg
            {...props}
            xmlns="http://www.w3.org/2000/svg"
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
        >
            <rect width="18" height="18" x="3" y="3" rx="2"/>
            <path d="m9 12 2 2 4-4"/>
        </svg>
    )
}


function UsersIcon(props: JSX.IntrinsicAttributes & SVGProps<SVGSVGElement>) {
    return (
        <svg
            {...props}
            xmlns="http://www.w3.org/2000/svg"
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
        >
            <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/>
            <circle cx="9" cy="7" r="4"/>
            <path d="M22 21v-2a4 4 0 0 0-3-3.87"/>
            <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
        </svg>
    )
}