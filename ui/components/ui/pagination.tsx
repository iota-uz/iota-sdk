import * as React from "react"
import {ChevronLeft, ChevronRight, MoreHorizontal} from "lucide-react"

import {cn} from "@/lib/utils"
import {ButtonProps, buttonVariants} from "@/components/ui/button"
import Link from "next/link";

type PaginationProps = {
    total: number;
    perPage: number;
    currentPage: number;
    onChange: (page: number) => void;
}

const Pagination = ({
                        total,
                        perPage,
                        currentPage,
                        className,
                        onChange,
                        ...props
                    }: PaginationProps & Omit<React.ComponentProps<"nav">, "onChange">) => {
    const lastPage = Math.ceil(total / perPage);

    const handlePageChange = (newPage: number) => {
        if (newPage > 0 && newPage <= lastPage) {
            onChange(newPage);
        }
    };

    const renderPaginationItems = () => {
        const pages = [
            <PaginationPrevious key="prev" onClick={() => handlePageChange(currentPage - 1)}
                                isActive={currentPage !== 1} href={`?page=${currentPage - 1}`}/>
        ];

        if (currentPage > 3) {
            pages.push(<PaginationEllipsis key="first-ellipsis"/>);
        }

        for (let page = Math.max(1, currentPage - 2); page <= Math.min(lastPage, currentPage + 2); page++) {
            pages.push(
                <PaginationLink href={`?page=${page}`} key={page} onClick={() => handlePageChange(page)}
                                isActive={currentPage === page}>
                    {page}
                </PaginationLink>
            );
        }

        if (currentPage < lastPage - 2) {
            pages.push(<PaginationEllipsis key="last-ellipsis"/>);
        }

        pages.push(
            <PaginationNext key="next" onClick={() => handlePageChange(currentPage + 1)}
                            isActive={currentPage !== lastPage} href={`?page=${currentPage + 1}`}/>
        );

        return pages;
    };

    return (
        <nav
            role="navigation"
            aria-label="pagination"
            className={cn("mx-auto flex w-full justify-center", className)}
            {...props}
        >
            {renderPaginationItems()}
        </nav>
    );
};
Pagination.displayName = "Pagination"

const PaginationContent = React.forwardRef<
    HTMLUListElement,
    React.ComponentProps<"ul">
>(({className, ...props}, ref) => (
    <ul
        ref={ref}
        className={cn("flex flex-row items-center gap-1", className)}
        {...props}
    />
))
PaginationContent.displayName = "PaginationContent"

const PaginationItem = React.forwardRef<
    HTMLLIElement,
    React.ComponentProps<"li">
>(({className, ...props}, ref) => (
    <li ref={ref} className={cn("", className)} {...props} />
))
PaginationItem.displayName = "PaginationItem"

type PaginationLinkProps = {
    isActive?: boolean
} & Pick<ButtonProps, "size"> &
    React.ComponentProps<typeof Link>

const PaginationLink = ({
                            className,
                            isActive,
                            size = "icon",
                            ...props
                        }: PaginationLinkProps) => (
    <Link
        aria-current={isActive ? "page" : undefined}
        className={cn(
            buttonVariants({
                variant: isActive ? "outline" : "ghost",
                size,
            }),
            className
        )}
        {...props}
    />
)
PaginationLink.displayName = "PaginationLink"

const PaginationPrevious = ({
                                className,
                                ...props
                            }: React.ComponentProps<typeof PaginationLink>) => (
    <PaginationLink
        aria-label="Go to previous page"
        size="default"
        className={cn("gap-1 pl-2.5", className)}
        {...props}
    >
        <ChevronLeft className="h-4 w-4"/>
        <span>Назад</span>
    </PaginationLink>
)
PaginationPrevious.displayName = "PaginationPrevious"

const PaginationNext = ({
                            className,
                            ...props
                        }: React.ComponentProps<typeof PaginationLink>) => (
    <PaginationLink
        aria-label="Go to next page"
        size="default"
        className={cn("gap-1 pr-2.5", className)}
        {...props}
    >
        <span>След</span>
        <ChevronRight className="h-4 w-4"/>
    </PaginationLink>
)
PaginationNext.displayName = "PaginationNext"

const PaginationEllipsis = ({
                                className,
                                ...props
                            }: React.ComponentProps<"span">) => (
    <span
        aria-hidden
        className={cn("flex h-9 w-9 items-center justify-center", className)}
        {...props}
    >
    <MoreHorizontal className="h-4 w-4"/>
    <span className="sr-only">More pages</span>
  </span>
)
PaginationEllipsis.displayName = "PaginationEllipsis"

export {
    Pagination,
    PaginationContent,
    PaginationEllipsis,
    PaginationItem,
    PaginationLink,
    PaginationNext,
    PaginationPrevious,
}
