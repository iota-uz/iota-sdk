import Link from "next/link"
import Image from "next/image"
import {useState} from "react";
import * as Icons from "@phosphor-icons/react";
import {Icon} from "@phosphor-icons/react";
import {cn} from "@/lib/utils";
import {usePathname} from "next/navigation";

type LinkProps = {
    name: string;
    href?: string;
    children?: LinkProps[];
    icon: Icon;
};

type SidebarProps = {
    links: LinkProps[];
};

type SidebarLinkProps = {
    name: string;
    icon: Icon;
    isExpanded: boolean;
    active: boolean;
} & LinkProps & React.ComponentProps<"button">;

export function SidebarLink({className, isExpanded, name, icon: PropIcon, href, active, ...props}: SidebarLinkProps) {
    if (href) {
        return (
            <Link href={href}
                  className={cn(
                      "flex items-center gap-4 px-4 py-3 text-base font-medium hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-50",
                      active ? "text-white hover:text-gray-100" : "text-gray-700 hover:bg-gray-100",
                      className,
                  )}>
                <PropIcon className="h-6 w-6"/>
                {name}
            </Link>
        )
    }
    return (
        <button
            {...props}
            className="flex items-center gap-4 rounded-md w-full px-4 py-3 text-base font-medium text-gray-700 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-50">
            <PropIcon className="h-6 w-6"/>
            {name}
            <span className="flex-1"/>
            <Icons.CaretUp
                className={cn("transition-transform duration-300 ease-in-out", isExpanded ? "rotate-180" : "")}
            />
        </button>
    )
}

export function SidebarItem(props: LinkProps) {
    const pathname = usePathname();
    const active = pathname == props.href;
    const [isExpanded, setIsExpanded] = useState(false);
    return (
        <li onClick={() => setIsExpanded(!isExpanded)} className={cn("rounded-md", active ? "bg-primary" : "")}>
            <SidebarLink name={props.name} href={props.href} icon={props.icon} isExpanded={isExpanded} active={active}/>
            {isExpanded && (
                <ul className="space-y-2 pl-4">
                    {props.children && props.children.map((link) => (
                        <SidebarItem key={link.name} {...link}/>
                    ))}
                </ul>
            )}
        </li>
    )
}

export function Sidebar({links}: SidebarProps) {
    return (
        <div key="1" className="flex w-full flex-col bg-white shadow-lg dark:bg-gray-950">
            <div className="flex h-16 items-center justify-center">
                <Link className="flex items-center gap-2" href="#">
                    <Image src={"/svg/iota-4c.svg"} alt="Logo" width={180} height={30}/>
                </Link>
            </div>
            <nav className="flex-1 overflow-y-auto px-2 py-4">
                <ul className="space-y-2">
                    {links.map((link) => (
                        <SidebarItem key={link.name} {...link}/>
                    ))}
                    <hr className="border-gray-200 dark:border-gray-700"/>
                    <SidebarLink className={"mt-4"} name="Выход" icon={Icons.SignOut} href={"/logout"}
                                 active={false}
                                 isExpanded={false}/>
                </ul>
            </nav>
        </div>
    )
}
