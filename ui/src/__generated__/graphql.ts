/* eslint-disable */
export type Maybe<T> = T | null;
export type InputMaybe<T> = Maybe<T>;
export type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
export type MakeOptional<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]?: Maybe<T[SubKey]> };
export type MakeMaybe<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]: Maybe<T[SubKey]> };
export type MakeEmpty<T extends { [key: string]: unknown }, K extends keyof T> = { [_ in K]?: never };
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: { input: string; output: string; }
  String: { input: string; output: string; }
  Boolean: { input: boolean; output: boolean; }
  Int: { input: number; output: number; }
  Float: { input: number; output: number; }
  Time: { input: any; output: any; }
};

export type AuthenticationLog = {
  __typename?: 'AuthenticationLog';
  createdAt: Scalars['Time']['output'];
  id: Scalars['ID']['output'];
  ip: Scalars['String']['output'];
  userAgent: Scalars['String']['output'];
  userId: Scalars['Int']['output'];
};

export type CreateExpense = {
  amount: Scalars['Float']['input'];
  categoryId: Scalars['ID']['input'];
  date?: InputMaybe<Scalars['String']['input']>;
};

export type CreateExpenseCategory = {
  amount: Scalars['Float']['input'];
  description?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
};

export type CreateRole = {
  description?: InputMaybe<Scalars['String']['input']>;
  name: Scalars['String']['input'];
};

export type CreateRolePermission = {
  permissionId: Scalars['Int']['input'];
  roleId: Scalars['Int']['input'];
};

export type CreateUser = {
  avatarId?: InputMaybe<Scalars['Int']['input']>;
  email: Scalars['String']['input'];
  employeeId?: InputMaybe<Scalars['Int']['input']>;
  firstName: Scalars['String']['input'];
  lastName: Scalars['String']['input'];
  password?: InputMaybe<Scalars['String']['input']>;
};

export type Employee = {
  __typename?: 'Employee';
  avatarId?: Maybe<Scalars['Int']['output']>;
  coefficient: Scalars['Float']['output'];
  createdAt: Scalars['Time']['output'];
  email: Scalars['String']['output'];
  firstName: Scalars['String']['output'];
  hourlyRate: Scalars['Float']['output'];
  id: Scalars['ID']['output'];
  lastName: Scalars['String']['output'];
  meta?: Maybe<EmployeeMeta>;
  middleName?: Maybe<Scalars['String']['output']>;
  phone?: Maybe<Scalars['String']['output']>;
  position?: Maybe<Position>;
  positionId: Scalars['Int']['output'];
  salary: Scalars['Float']['output'];
  updatedAt: Scalars['Time']['output'];
};

export type EmployeeMeta = {
  __typename?: 'EmployeeMeta';
  birthDate?: Maybe<Scalars['Time']['output']>;
  employeeId: Scalars['Int']['output'];
  generalInfo?: Maybe<Scalars['String']['output']>;
  joinDate?: Maybe<Scalars['Time']['output']>;
  leaveDate?: Maybe<Scalars['Time']['output']>;
  primaryLanguage?: Maybe<Scalars['String']['output']>;
  secondaryLanguage?: Maybe<Scalars['String']['output']>;
  tin?: Maybe<Scalars['String']['output']>;
  updatedAt: Scalars['Time']['output'];
  ytProfileId?: Maybe<Scalars['String']['output']>;
};

export type Expense = {
  __typename?: 'Expense';
  amount: Scalars['Float']['output'];
  category?: Maybe<ExpenseCategory>;
  categoryId: Scalars['ID']['output'];
  createdAt: Scalars['Time']['output'];
  date: Scalars['Time']['output'];
  id: Scalars['ID']['output'];
  updatedAt: Scalars['Time']['output'];
};

export type ExpenseCategory = {
  __typename?: 'ExpenseCategory';
  amount: Scalars['Float']['output'];
  createdAt: Scalars['Time']['output'];
  description?: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  name: Scalars['String']['output'];
  updatedAt: Scalars['Time']['output'];
};

export type Mutation = {
  __typename?: 'Mutation';
  authenticate: Session;
  createExpense: Expense;
  createExpenseCategory: ExpenseCategory;
  createRole: Role;
  createRolePermission: RolePermissions;
  createUser: User;
  deleteExpense: Scalars['Boolean']['output'];
  deleteExpenseCategory: Scalars['Boolean']['output'];
  deleteRole: Scalars['Boolean']['output'];
  deleteSession: Scalars['Boolean']['output'];
  deleteUser: Scalars['Boolean']['output'];
  updateExpense: Expense;
  updateExpenseCategory: ExpenseCategory;
  updateRole: Role;
  updateUser: User;
};


export type MutationAuthenticateArgs = {
  email: Scalars['String']['input'];
  password: Scalars['String']['input'];
};


export type MutationCreateExpenseArgs = {
  input: CreateExpense;
};


export type MutationCreateExpenseCategoryArgs = {
  input: CreateExpenseCategory;
};


export type MutationCreateRoleArgs = {
  input: CreateRole;
};


export type MutationCreateRolePermissionArgs = {
  input: CreateRolePermission;
};


export type MutationCreateUserArgs = {
  input: CreateUser;
};


export type MutationDeleteExpenseArgs = {
  id: Scalars['ID']['input'];
};


export type MutationDeleteExpenseCategoryArgs = {
  id: Scalars['ID']['input'];
};


export type MutationDeleteRoleArgs = {
  id: Scalars['ID']['input'];
};


export type MutationDeleteSessionArgs = {
  token: Scalars['String']['input'];
};


export type MutationDeleteUserArgs = {
  id: Scalars['ID']['input'];
};


export type MutationUpdateExpenseArgs = {
  id: Scalars['ID']['input'];
  input: UpdateExpense;
};


export type MutationUpdateExpenseCategoryArgs = {
  id: Scalars['ID']['input'];
  input: UpdateExpenseCategory;
};


export type MutationUpdateRoleArgs = {
  id: Scalars['ID']['input'];
  input: UpdateRole;
};


export type MutationUpdateUserArgs = {
  id: Scalars['ID']['input'];
  input: UpdateUser;
};

export type PaginatedAuthenticationLogs = {
  __typename?: 'PaginatedAuthenticationLogs';
  data: Array<AuthenticationLog>;
  total: Scalars['Int']['output'];
};

export type PaginatedEmployees = {
  __typename?: 'PaginatedEmployees';
  data: Array<Employee>;
  total: Scalars['Int']['output'];
};

export type PaginatedExpenseCategories = {
  __typename?: 'PaginatedExpenseCategories';
  data: Array<ExpenseCategory>;
  total: Scalars['Int']['output'];
};

export type PaginatedExpenses = {
  __typename?: 'PaginatedExpenses';
  data: Array<Expense>;
  total: Scalars['Int']['output'];
};

export type PaginatedPermissions = {
  __typename?: 'PaginatedPermissions';
  data: Array<Permission>;
  total: Scalars['Int']['output'];
};

export type PaginatedPositions = {
  __typename?: 'PaginatedPositions';
  data: Array<Position>;
  total: Scalars['Int']['output'];
};

export type PaginatedRolePermissions = {
  __typename?: 'PaginatedRolePermissions';
  data: Array<RolePermissions>;
  total: Scalars['Int']['output'];
};

export type PaginatedRoles = {
  __typename?: 'PaginatedRoles';
  data: Array<Role>;
  total: Scalars['Int']['output'];
};

export type PaginatedSessions = {
  __typename?: 'PaginatedSessions';
  data: Array<Session>;
  total: Scalars['Int']['output'];
};

export type PaginatedUploads = {
  __typename?: 'PaginatedUploads';
  data: Array<Upload>;
  total: Scalars['Int']['output'];
};

export type PaginatedUsers = {
  __typename?: 'PaginatedUsers';
  data: Array<User>;
  total: Scalars['Int']['output'];
};

export type Permission = {
  __typename?: 'Permission';
  action?: Maybe<Scalars['String']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  modifier?: Maybe<Scalars['String']['output']>;
  resource?: Maybe<Scalars['String']['output']>;
};

export type Position = {
  __typename?: 'Position';
  createdAt: Scalars['Time']['output'];
  description?: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  name: Scalars['String']['output'];
  updatedAt: Scalars['Time']['output'];
};

export type Query = {
  __typename?: 'Query';
  authenticationLog?: Maybe<AuthenticationLog>;
  authenticationLogs: PaginatedAuthenticationLogs;
  employee?: Maybe<Employee>;
  employees: PaginatedEmployees;
  expense?: Maybe<Expense>;
  expenseCategories: PaginatedExpenseCategories;
  expenseCategory?: Maybe<ExpenseCategory>;
  expenses: PaginatedExpenses;
  permission?: Maybe<Permission>;
  permissions: PaginatedPermissions;
  position?: Maybe<Position>;
  positions: PaginatedPositions;
  role?: Maybe<Role>;
  rolePermission?: Maybe<RolePermissions>;
  rolePermissions: PaginatedRolePermissions;
  roles: PaginatedRoles;
  session?: Maybe<Session>;
  sessions: PaginatedSessions;
  upload?: Maybe<Upload>;
  uploads: PaginatedUploads;
  user?: Maybe<User>;
  users: PaginatedUsers;
};


export type QueryAuthenticationLogArgs = {
  id: Scalars['ID']['input'];
};


export type QueryAuthenticationLogsArgs = {
  limit: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryEmployeeArgs = {
  id: Scalars['ID']['input'];
};


export type QueryEmployeesArgs = {
  limit: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryExpenseArgs = {
  id: Scalars['ID']['input'];
};


export type QueryExpenseCategoriesArgs = {
  limit: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryExpenseCategoryArgs = {
  id: Scalars['ID']['input'];
};


export type QueryExpensesArgs = {
  limit: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryPermissionArgs = {
  id: Scalars['ID']['input'];
};


export type QueryPermissionsArgs = {
  limit: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryPositionArgs = {
  id: Scalars['ID']['input'];
};


export type QueryPositionsArgs = {
  limit: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryRoleArgs = {
  id: Scalars['ID']['input'];
};


export type QueryRolePermissionArgs = {
  permissionId: Scalars['Int']['input'];
  roleId: Scalars['Int']['input'];
};


export type QueryRolePermissionsArgs = {
  limit: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryRolesArgs = {
  limit: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QuerySessionArgs = {
  token: Scalars['String']['input'];
};


export type QuerySessionsArgs = {
  limit: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryUploadArgs = {
  id: Scalars['ID']['input'];
};


export type QueryUploadsArgs = {
  limit: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryUserArgs = {
  id: Scalars['ID']['input'];
};


export type QueryUsersArgs = {
  limit: Scalars['Int']['input'];
  offset: Scalars['Int']['input'];
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};

export type Role = {
  __typename?: 'Role';
  createdAt: Scalars['Time']['output'];
  description?: Maybe<Scalars['String']['output']>;
  id: Scalars['ID']['output'];
  name: Scalars['String']['output'];
  updatedAt: Scalars['Time']['output'];
};

export type RolePermissions = {
  __typename?: 'RolePermissions';
  permissionId: Scalars['Int']['output'];
  roleId: Scalars['Int']['output'];
};

export type Session = {
  __typename?: 'Session';
  createdAt: Scalars['Time']['output'];
  expiresAt: Scalars['Time']['output'];
  ip: Scalars['String']['output'];
  token: Scalars['String']['output'];
  userAgent: Scalars['String']['output'];
  userId: Scalars['Int']['output'];
};

export type UpdateExpense = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  categoryId?: InputMaybe<Scalars['ID']['input']>;
  date?: InputMaybe<Scalars['String']['input']>;
};

export type UpdateExpenseCategory = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
};

export type UpdateRole = {
  description?: InputMaybe<Scalars['String']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
};

export type UpdateUser = {
  avatarId?: InputMaybe<Scalars['Int']['input']>;
  email?: InputMaybe<Scalars['String']['input']>;
  employeeId?: InputMaybe<Scalars['Int']['input']>;
  firstName?: InputMaybe<Scalars['String']['input']>;
  lastName?: InputMaybe<Scalars['String']['input']>;
  password?: InputMaybe<Scalars['String']['input']>;
};

export type Upload = {
  __typename?: 'Upload';
  createdAt: Scalars['Time']['output'];
  id: Scalars['ID']['output'];
  mimetype: Scalars['String']['output'];
  name: Scalars['String']['output'];
  path: Scalars['String']['output'];
  size: Scalars['Float']['output'];
  updatedAt: Scalars['Time']['output'];
  uploaderId?: Maybe<Scalars['ID']['output']>;
};

export type User = {
  __typename?: 'User';
  avatar?: Maybe<Upload>;
  avatarId?: Maybe<Scalars['ID']['output']>;
  createdAt: Scalars['Time']['output'];
  email: Scalars['String']['output'];
  employeeId?: Maybe<Scalars['ID']['output']>;
  firstName: Scalars['String']['output'];
  id: Scalars['ID']['output'];
  lastAction?: Maybe<Scalars['Time']['output']>;
  lastIp?: Maybe<Scalars['String']['output']>;
  lastLogin?: Maybe<Scalars['Time']['output']>;
  lastName: Scalars['String']['output'];
  middleName?: Maybe<Scalars['String']['output']>;
  updatedAt: Scalars['Time']['output'];
};
