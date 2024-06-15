'use client';
import {Input} from "@/components/ui/input"
import {Button} from "@/components/ui/button"
import {DropdownMenu, DropdownMenuContent, DropdownMenuTrigger} from "@/components/ui/dropdown-menu"
import {Label} from "@/components/ui/label"
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select"
import {Popover, PopoverContent, PopoverTrigger} from "@/components/ui/popover"
import {Calendar} from "@/components/ui/calendar"
import {Pagination} from "@/components/ui/pagination"
import * as Icons from "@phosphor-icons/react"
import BaseTable, {Column, SortBy} from "@/components/composite/table";
import {gql, useQuery} from "@apollo/client";
import React, {useEffect, useState} from "react";
import PerPageSelect from "@/components/ui/per-page-select";
import Link from "next/link";
import {Card} from "@/components/ui/card";
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar";
import {User} from "@/src/__generated__/graphql";

const GET_USERS = gql`
    query GetUsers($limit: Int!, $offset: Int!, $sortBy: [String!]) {
        users(limit: $limit, offset: $offset, sortBy: $sortBy) {
            total
            data {
                id
                firstName
                lastName
                email
                createdAt
                updatedAt
                avatar {
                    id
                }
            }
        }
    }
`;

export default function Component() {
    const [perPage, setPerPage] = useState(10);
    const [page, setPage] = useState(1);
    const [sortBy, setSortBy] = useState<SortBy<User>>({
        createdAt: 'desc'
    });
    const {data, loading, error, refetch} = useQuery(GET_USERS, {
        variables: {
            limit: perPage,
            offset: (page - 1) * perPage,
            sortBy: Object.keys(sortBy).map((key) => `${key} ${sortBy[key as keyof User]}`)
        }
    });
    useEffect(() => {
        refetch({
            limit: perPage,
            offset: (page - 1) * perPage,
            sortBy: Object.keys(sortBy).map((key) => `${key} ${sortBy[key as keyof User]}`)
        });
    }, [page, perPage, sortBy]);

    const {total, data: users} = data?.users || {total: 0, data: []};
    if (error) {
        return (
            <div className="container mx-auto px-8 py-8">
                <h1 className="text-2xl">Ошибка при загрузке пользователей</h1>
                <p className="text-lg">
                    {error.message}
                </p>
                <code>
                    {error.stack}
                </code>
                <div className="flex justify-center mt-4">
                    <Button onClick={() => refetch()}>
                        Повторить
                    </Button>
                </div>
            </div>
        );
    }

    const columns: Array<Column<User>> = [
        {
            label: 'ФИО',
            key: 'firstName',
            field: (item) => (
                <div className="flex items-center gap-2.5">
                    <Avatar className="w-10 h-10">
                        <AvatarImage
                            className="object-cover"
                            src={"https://media.istockphoto.com/id/1416048929/photo/woman-working-on-laptop-online-checking-emails-and-planning-on-the-internet-while-sitting-in.jpg?s=612x612&w=0&k=20&c=mt-Bsap56B_7Lgx1fcLqFVXTeDbIOILVjTdOqrDS54s="}
                            alt="Автар пользователя"
                        />
                        <AvatarFallback>CN</AvatarFallback>
                    </Avatar>
                    <div>
                        <p>{item.firstName} {item.lastName} {item.middleName}</p>
                        <p className="text-gray-400">{item.email}</p>
                    </div>
                </div>
            ),
        },
        {
            label: 'Последнее действие',
            key: 'lastAction',
            dateFormat: 'calendar',
            sortable: true,
        },
        {
            label: 'Дата создания',
            key: 'createdAt',
            dateFormat: 'calendar',
            sortable: true,
        },
        {
            label: 'Дата обновления',
            key: 'updatedAt',
            dateFormat: 'calendar',
            sortable: true,
        },
        {
            label: '',
            key: 'actions',
            width: 8,
            field: (item) => (
                <div className="flex justify-center">
                    <Link href={`/users/${item.id}`}>
                        <Button size="sm" variant="outline" className="text-gray-700 bg-primary-100 p-3">
                            <Icons.PencilSimple size={18} weight="bold"/>
                        </Button>
                    </Link>
                </div>
            )
        }
    ];
    return (
        <div className="container mx-auto px-8 py-8">
            <div className="flex items-center justify-between mb-6">
                <div>
                    <h1 className="text-2xl">
                        Пользователи
                    </h1>
                    <h2 className="text-gray-500">
                        Пользователи могут авторизоваться в системе
                    </h2>
                </div>
                <div className="flex items-center gap-4">
                    <div className="flex items-center gap-4">
                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button size="sm">
                                    <Icons.Funnel className="w-4 h-4 mr-2"/>
                                    Фильтры
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent className="w-56 p-4 space-y-2">
                                <div className="space-y-2">
                                    <Label htmlFor="role-filter">Role</Label>
                                    <Select defaultValue="all">
                                        <SelectTrigger className="w-full">
                                            <SelectValue placeholder="All"/>
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="all">All</SelectItem>
                                            <SelectItem value="admin">Admin</SelectItem>
                                            <SelectItem value="user">User</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="date-filter">Created At</Label>
                                    <Popover>
                                        <PopoverTrigger asChild>
                                            <Button className="w-full flex-col h-auto items-start" variant="outline">
                                                <span className="font-semibold uppercase text-[0.65rem]">Select Date Range</span>
                                                <span className="font-normal"/>
                                            </Button>
                                        </PopoverTrigger>
                                        <PopoverContent className="p-0 max-w-[276px]">
                                            <Calendar mode="range"/>
                                        </PopoverContent>
                                    </Popover>
                                </div>
                                <div className="flex justify-end gap-2">
                                    <Button size="sm" variant="outline">
                                        Clear
                                    </Button>
                                    <Button size="sm">Apply</Button>
                                </div>
                            </DropdownMenuContent>
                        </DropdownMenu>
                        <Link href={"/users/new"}>
                            <Button size="sm">
                                <Icons.PlusCircle className="w-4 h-4 mr-2"/>
                                Новый пользователь
                            </Button>
                        </Link>
                    </div>
                </div>
            </div>
            <Card className="rounded-lg">
                <div className="p-5">
                    <div className="relative">
                        <Icons.MagnifyingGlass
                            className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-500 dark:text-gray-400"/>
                        <Input
                            className="pl-10 pr-4 py-2 rounded-md bg-white shadow-sm dark:bg-gray-800 dark:text-gray-200"
                            placeholder="Поиск..."
                            type="text"
                        />
                    </div>
                </div>
                <hr className="bg-primary-100"/>
                <div className="p-5">
                    <BaseTable
                        primaryKey={"id"}
                        columns={columns}
                        data={users}
                        sortBy={sortBy}
                        loading={loading}
                        onSort={setSortBy}
                    />
                    <div className="flex items-center justify-between mt-6">
                        <Pagination total={total} perPage={perPage} currentPage={page} onChange={setPage}/>
                        <div className="flex items-center gap-2">
                    <span className="text-sm text-gray-500 dark:text-gray-400">
                        {`Showing ${Math.min((page - 1) * perPage + 1, total)}-${Math.min(page * perPage, total)} of ${total} users`}
                    </span>
                            <PerPageSelect perPage={perPage} setPerPage={setPerPage} options={[10, 20, 50]}/>
                        </div>
                    </div>
                </div>
            </Card>
        </div>
    )
}