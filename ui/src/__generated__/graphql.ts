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
  /** The `DateTime` scalar type represents a DateTime. The DateTime is serialized as an RFC 3339 quoted string */
  DateTime: { input: any; output: any; }
};

export type AuthPayload = {
  __typename?: 'AuthPayload';
  token?: Maybe<Scalars['String']['output']>;
};

export type AvatarCreateInput = {
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  mimetype?: InputMaybe<Scalars['String']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  path?: InputMaybe<Scalars['String']['input']>;
  size?: InputMaybe<Scalars['Float']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
  uploader_id?: InputMaybe<Scalars['Int']['input']>;
};

export type AvatarUpdateInput = {
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  mimetype?: InputMaybe<Scalars['String']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  path?: InputMaybe<Scalars['String']['input']>;
  size?: InputMaybe<Scalars['Float']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
  uploader_id?: InputMaybe<Scalars['Int']['input']>;
};

export type CategoryCreateInput = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type CategoryUpdateInput = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type CreateEmployeeInput = {
  avatar_id?: InputMaybe<Scalars['Int']['input']>;
  coefficient?: InputMaybe<Scalars['Float']['input']>;
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  email?: InputMaybe<Scalars['String']['input']>;
  first_name?: InputMaybe<Scalars['String']['input']>;
  hourly_rate?: InputMaybe<Scalars['Float']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  last_name?: InputMaybe<Scalars['String']['input']>;
  meta?: InputMaybe<MetaCreateInput>;
  middle_name?: InputMaybe<Scalars['String']['input']>;
  phone?: InputMaybe<Scalars['String']['input']>;
  position?: InputMaybe<PositionCreateInput>;
  position_id?: InputMaybe<Scalars['Int']['input']>;
  salary?: InputMaybe<Scalars['Float']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type CreateExpenseCategoryInput = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type CreateExpenseInput = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  category?: InputMaybe<CategoryCreateInput>;
  category_id?: InputMaybe<Scalars['Int']['input']>;
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  date?: InputMaybe<Scalars['DateTime']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type CreatePositionInput = {
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type CreateTaskTypeInput = {
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  icon?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type CreateUserInput = {
  avatar?: InputMaybe<AvatarCreateInput>;
  avatarId?: InputMaybe<Scalars['Int']['input']>;
  createdAt?: InputMaybe<Scalars['DateTime']['input']>;
  email?: InputMaybe<Scalars['String']['input']>;
  employeeId?: InputMaybe<Scalars['Int']['input']>;
  firstName?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  lastAction?: InputMaybe<Scalars['DateTime']['input']>;
  lastIp?: InputMaybe<Scalars['String']['input']>;
  lastLogin?: InputMaybe<Scalars['DateTime']['input']>;
  lastName?: InputMaybe<Scalars['String']['input']>;
  middleName?: InputMaybe<Scalars['String']['input']>;
  password?: InputMaybe<Scalars['String']['input']>;
  updatedAt?: InputMaybe<Scalars['DateTime']['input']>;
};

export type Employee = {
  __typename?: 'Employee';
  avatar_id?: Maybe<Scalars['Int']['output']>;
  coefficient?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['DateTime']['output']>;
  email?: Maybe<Scalars['String']['output']>;
  first_name?: Maybe<Scalars['String']['output']>;
  hourly_rate?: Maybe<Scalars['Float']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  last_name?: Maybe<Scalars['String']['output']>;
  meta?: Maybe<EmployeemetaJoin>;
  middle_name?: Maybe<Scalars['String']['output']>;
  phone?: Maybe<Scalars['String']['output']>;
  position?: Maybe<EmployeepositionJoin>;
  position_id?: Maybe<Scalars['Int']['output']>;
  salary?: Maybe<Scalars['Float']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type EmployeeType = {
  __typename?: 'EmployeeType';
  avatar_id?: Maybe<Scalars['Int']['output']>;
  coefficient?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['DateTime']['output']>;
  email?: Maybe<Scalars['String']['output']>;
  first_name?: Maybe<Scalars['String']['output']>;
  hourly_rate?: Maybe<Scalars['Float']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  last_name?: Maybe<Scalars['String']['output']>;
  meta?: Maybe<EmployeeTypemetaJoin>;
  middle_name?: Maybe<Scalars['String']['output']>;
  phone?: Maybe<Scalars['String']['output']>;
  position?: Maybe<EmployeeTypepositionJoin>;
  position_id?: Maybe<Scalars['Int']['output']>;
  salary?: Maybe<Scalars['Float']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type EmployeeTypemetaJoin = {
  __typename?: 'EmployeeTypemetaJoin';
  birth_date?: Maybe<Scalars['DateTime']['output']>;
  employee_id?: Maybe<Scalars['Int']['output']>;
  general_info?: Maybe<Scalars['String']['output']>;
  join_date?: Maybe<Scalars['DateTime']['output']>;
  leave_date?: Maybe<Scalars['DateTime']['output']>;
  primary_language?: Maybe<Scalars['String']['output']>;
  secondary_language?: Maybe<Scalars['String']['output']>;
  tin?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
  yt_profile_id?: Maybe<Scalars['String']['output']>;
};

export type EmployeeTypepositionJoin = {
  __typename?: 'EmployeeTypepositionJoin';
  created_at?: Maybe<Scalars['DateTime']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type EmployeemetaJoin = {
  __typename?: 'EmployeemetaJoin';
  birth_date?: Maybe<Scalars['DateTime']['output']>;
  employee_id?: Maybe<Scalars['Int']['output']>;
  general_info?: Maybe<Scalars['String']['output']>;
  join_date?: Maybe<Scalars['DateTime']['output']>;
  leave_date?: Maybe<Scalars['DateTime']['output']>;
  primary_language?: Maybe<Scalars['String']['output']>;
  secondary_language?: Maybe<Scalars['String']['output']>;
  tin?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
  yt_profile_id?: Maybe<Scalars['String']['output']>;
};

export type EmployeepositionJoin = {
  __typename?: 'EmployeepositionJoin';
  created_at?: Maybe<Scalars['DateTime']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type EmployeesAggregateAggregate = {
  __typename?: 'EmployeesAggregateAggregate';
  AvatarId?: Maybe<EmployeesAggregateAvatarIdAggregationQuery>;
  Coefficient?: Maybe<EmployeesAggregateCoefficientAggregationQuery>;
  CreatedAt?: Maybe<EmployeesAggregateCreatedAtAggregationQuery>;
  Email?: Maybe<EmployeesAggregateEmailAggregationQuery>;
  FirstName?: Maybe<EmployeesAggregateFirstNameAggregationQuery>;
  HourlyRate?: Maybe<EmployeesAggregateHourlyRateAggregationQuery>;
  Id?: Maybe<EmployeesAggregateIdAggregationQuery>;
  LastName?: Maybe<EmployeesAggregateLastNameAggregationQuery>;
  MiddleName?: Maybe<EmployeesAggregateMiddleNameAggregationQuery>;
  Phone?: Maybe<EmployeesAggregatePhoneAggregationQuery>;
  PositionId?: Maybe<EmployeesAggregatePositionIdAggregationQuery>;
  Salary?: Maybe<EmployeesAggregateSalaryAggregationQuery>;
  UpdatedAt?: Maybe<EmployeesAggregateUpdatedAtAggregationQuery>;
};


export type EmployeesAggregateAggregateAvatarIdArgs = {
  gt?: InputMaybe<Scalars['Int']['input']>;
  gte?: InputMaybe<Scalars['Int']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  lt?: InputMaybe<Scalars['Int']['input']>;
  lte?: InputMaybe<Scalars['Int']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type EmployeesAggregateAggregateCoefficientArgs = {
  gt?: InputMaybe<Scalars['Float']['input']>;
  gte?: InputMaybe<Scalars['Float']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
  lt?: InputMaybe<Scalars['Float']['input']>;
  lte?: InputMaybe<Scalars['Float']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
};


export type EmployeesAggregateAggregateCreatedAtArgs = {
  gt?: InputMaybe<Scalars['DateTime']['input']>;
  gte?: InputMaybe<Scalars['DateTime']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
  lt?: InputMaybe<Scalars['DateTime']['input']>;
  lte?: InputMaybe<Scalars['DateTime']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
};


export type EmployeesAggregateAggregateEmailArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type EmployeesAggregateAggregateFirstNameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type EmployeesAggregateAggregateHourlyRateArgs = {
  gt?: InputMaybe<Scalars['Float']['input']>;
  gte?: InputMaybe<Scalars['Float']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
  lt?: InputMaybe<Scalars['Float']['input']>;
  lte?: InputMaybe<Scalars['Float']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
};


export type EmployeesAggregateAggregateIdArgs = {
  gt?: InputMaybe<Scalars['Int']['input']>;
  gte?: InputMaybe<Scalars['Int']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  lt?: InputMaybe<Scalars['Int']['input']>;
  lte?: InputMaybe<Scalars['Int']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type EmployeesAggregateAggregateLastNameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type EmployeesAggregateAggregateMiddleNameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type EmployeesAggregateAggregatePhoneArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type EmployeesAggregateAggregatePositionIdArgs = {
  gt?: InputMaybe<Scalars['Int']['input']>;
  gte?: InputMaybe<Scalars['Int']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  lt?: InputMaybe<Scalars['Int']['input']>;
  lte?: InputMaybe<Scalars['Int']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type EmployeesAggregateAggregateSalaryArgs = {
  gt?: InputMaybe<Scalars['Float']['input']>;
  gte?: InputMaybe<Scalars['Float']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
  lt?: InputMaybe<Scalars['Float']['input']>;
  lte?: InputMaybe<Scalars['Float']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
};


export type EmployeesAggregateAggregateUpdatedAtArgs = {
  gt?: InputMaybe<Scalars['DateTime']['input']>;
  gte?: InputMaybe<Scalars['DateTime']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
  lt?: InputMaybe<Scalars['DateTime']['input']>;
  lte?: InputMaybe<Scalars['DateTime']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
};

export type EmployeesAggregateAvatarIdAggregationQuery = {
  __typename?: 'EmployeesAggregateAvatarIdAggregationQuery';
  avg?: Maybe<Scalars['Int']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Int']['output']>;
  min?: Maybe<Scalars['Int']['output']>;
  sum?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesAggregateCoefficientAggregationQuery = {
  __typename?: 'EmployeesAggregateCoefficientAggregationQuery';
  avg?: Maybe<Scalars['Float']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Float']['output']>;
  min?: Maybe<Scalars['Float']['output']>;
  sum?: Maybe<Scalars['Float']['output']>;
};

export type EmployeesAggregateCreatedAtAggregationQuery = {
  __typename?: 'EmployeesAggregateCreatedAtAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['DateTime']['output']>;
  min?: Maybe<Scalars['DateTime']['output']>;
};

export type EmployeesAggregateEmailAggregationQuery = {
  __typename?: 'EmployeesAggregateEmailAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesAggregateFirstNameAggregationQuery = {
  __typename?: 'EmployeesAggregateFirstNameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesAggregateHourlyRateAggregationQuery = {
  __typename?: 'EmployeesAggregateHourlyRateAggregationQuery';
  avg?: Maybe<Scalars['Float']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Float']['output']>;
  min?: Maybe<Scalars['Float']['output']>;
  sum?: Maybe<Scalars['Float']['output']>;
};

export type EmployeesAggregateIdAggregationQuery = {
  __typename?: 'EmployeesAggregateIdAggregationQuery';
  avg?: Maybe<Scalars['Int']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Int']['output']>;
  min?: Maybe<Scalars['Int']['output']>;
  sum?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesAggregateLastNameAggregationQuery = {
  __typename?: 'EmployeesAggregateLastNameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesAggregateMiddleNameAggregationQuery = {
  __typename?: 'EmployeesAggregateMiddleNameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesAggregatePhoneAggregationQuery = {
  __typename?: 'EmployeesAggregatePhoneAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesAggregatePositionIdAggregationQuery = {
  __typename?: 'EmployeesAggregatePositionIdAggregationQuery';
  avg?: Maybe<Scalars['Int']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Int']['output']>;
  min?: Maybe<Scalars['Int']['output']>;
  sum?: Maybe<Scalars['Int']['output']>;
};

export type EmployeesAggregateSalaryAggregationQuery = {
  __typename?: 'EmployeesAggregateSalaryAggregationQuery';
  avg?: Maybe<Scalars['Float']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Float']['output']>;
  min?: Maybe<Scalars['Float']['output']>;
  sum?: Maybe<Scalars['Float']['output']>;
};

export type EmployeesAggregateUpdatedAtAggregationQuery = {
  __typename?: 'EmployeesAggregateUpdatedAtAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['DateTime']['output']>;
  min?: Maybe<Scalars['DateTime']['output']>;
};

export type Expense = {
  __typename?: 'Expense';
  amount?: Maybe<Scalars['Float']['output']>;
  category?: Maybe<ExpensecategoryJoin>;
  category_id?: Maybe<Scalars['Int']['output']>;
  created_at?: Maybe<Scalars['DateTime']['output']>;
  date?: Maybe<Scalars['DateTime']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type ExpenseCategoriesAggregateAggregate = {
  __typename?: 'ExpenseCategoriesAggregateAggregate';
  Amount?: Maybe<ExpenseCategoriesAggregateAmountAggregationQuery>;
  CreatedAt?: Maybe<ExpenseCategoriesAggregateCreatedAtAggregationQuery>;
  Description?: Maybe<ExpenseCategoriesAggregateDescriptionAggregationQuery>;
  Id?: Maybe<ExpenseCategoriesAggregateIdAggregationQuery>;
  Name?: Maybe<ExpenseCategoriesAggregateNameAggregationQuery>;
  UpdatedAt?: Maybe<ExpenseCategoriesAggregateUpdatedAtAggregationQuery>;
};


export type ExpenseCategoriesAggregateAggregateAmountArgs = {
  gt?: InputMaybe<Scalars['Float']['input']>;
  gte?: InputMaybe<Scalars['Float']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
  lt?: InputMaybe<Scalars['Float']['input']>;
  lte?: InputMaybe<Scalars['Float']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
};


export type ExpenseCategoriesAggregateAggregateCreatedAtArgs = {
  gt?: InputMaybe<Scalars['DateTime']['input']>;
  gte?: InputMaybe<Scalars['DateTime']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
  lt?: InputMaybe<Scalars['DateTime']['input']>;
  lte?: InputMaybe<Scalars['DateTime']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
};


export type ExpenseCategoriesAggregateAggregateDescriptionArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type ExpenseCategoriesAggregateAggregateIdArgs = {
  gt?: InputMaybe<Scalars['Int']['input']>;
  gte?: InputMaybe<Scalars['Int']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  lt?: InputMaybe<Scalars['Int']['input']>;
  lte?: InputMaybe<Scalars['Int']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type ExpenseCategoriesAggregateAggregateNameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type ExpenseCategoriesAggregateAggregateUpdatedAtArgs = {
  gt?: InputMaybe<Scalars['DateTime']['input']>;
  gte?: InputMaybe<Scalars['DateTime']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
  lt?: InputMaybe<Scalars['DateTime']['input']>;
  lte?: InputMaybe<Scalars['DateTime']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
};

export type ExpenseCategoriesAggregateAmountAggregationQuery = {
  __typename?: 'ExpenseCategoriesAggregateAmountAggregationQuery';
  avg?: Maybe<Scalars['Float']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Float']['output']>;
  min?: Maybe<Scalars['Float']['output']>;
  sum?: Maybe<Scalars['Float']['output']>;
};

export type ExpenseCategoriesAggregateCreatedAtAggregationQuery = {
  __typename?: 'ExpenseCategoriesAggregateCreatedAtAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['DateTime']['output']>;
  min?: Maybe<Scalars['DateTime']['output']>;
};

export type ExpenseCategoriesAggregateDescriptionAggregationQuery = {
  __typename?: 'ExpenseCategoriesAggregateDescriptionAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type ExpenseCategoriesAggregateIdAggregationQuery = {
  __typename?: 'ExpenseCategoriesAggregateIdAggregationQuery';
  avg?: Maybe<Scalars['Int']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Int']['output']>;
  min?: Maybe<Scalars['Int']['output']>;
  sum?: Maybe<Scalars['Int']['output']>;
};

export type ExpenseCategoriesAggregateNameAggregationQuery = {
  __typename?: 'ExpenseCategoriesAggregateNameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type ExpenseCategoriesAggregateUpdatedAtAggregationQuery = {
  __typename?: 'ExpenseCategoriesAggregateUpdatedAtAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['DateTime']['output']>;
  min?: Maybe<Scalars['DateTime']['output']>;
};

export type ExpenseCategory = {
  __typename?: 'ExpenseCategory';
  amount?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['DateTime']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type ExpenseCategoryType = {
  __typename?: 'ExpenseCategoryType';
  amount?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['DateTime']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type ExpenseType = {
  __typename?: 'ExpenseType';
  amount?: Maybe<Scalars['Float']['output']>;
  category?: Maybe<ExpenseTypecategoryJoin>;
  category_id?: Maybe<Scalars['Int']['output']>;
  created_at?: Maybe<Scalars['DateTime']['output']>;
  date?: Maybe<Scalars['DateTime']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type ExpenseTypecategoryJoin = {
  __typename?: 'ExpenseTypecategoryJoin';
  amount?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['DateTime']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type ExpensecategoryJoin = {
  __typename?: 'ExpensecategoryJoin';
  amount?: Maybe<Scalars['Float']['output']>;
  created_at?: Maybe<Scalars['DateTime']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type ExpensesAggregateAggregate = {
  __typename?: 'ExpensesAggregateAggregate';
  Amount?: Maybe<ExpensesAggregateAmountAggregationQuery>;
  CategoryId?: Maybe<ExpensesAggregateCategoryIdAggregationQuery>;
  CreatedAt?: Maybe<ExpensesAggregateCreatedAtAggregationQuery>;
  Date?: Maybe<ExpensesAggregateDateAggregationQuery>;
  Id?: Maybe<ExpensesAggregateIdAggregationQuery>;
  UpdatedAt?: Maybe<ExpensesAggregateUpdatedAtAggregationQuery>;
};


export type ExpensesAggregateAggregateAmountArgs = {
  gt?: InputMaybe<Scalars['Float']['input']>;
  gte?: InputMaybe<Scalars['Float']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
  lt?: InputMaybe<Scalars['Float']['input']>;
  lte?: InputMaybe<Scalars['Float']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Float']['input']>>>;
};


export type ExpensesAggregateAggregateCategoryIdArgs = {
  gt?: InputMaybe<Scalars['Int']['input']>;
  gte?: InputMaybe<Scalars['Int']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  lt?: InputMaybe<Scalars['Int']['input']>;
  lte?: InputMaybe<Scalars['Int']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type ExpensesAggregateAggregateCreatedAtArgs = {
  gt?: InputMaybe<Scalars['DateTime']['input']>;
  gte?: InputMaybe<Scalars['DateTime']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
  lt?: InputMaybe<Scalars['DateTime']['input']>;
  lte?: InputMaybe<Scalars['DateTime']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
};


export type ExpensesAggregateAggregateDateArgs = {
  gt?: InputMaybe<Scalars['DateTime']['input']>;
  gte?: InputMaybe<Scalars['DateTime']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
  lt?: InputMaybe<Scalars['DateTime']['input']>;
  lte?: InputMaybe<Scalars['DateTime']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
};


export type ExpensesAggregateAggregateIdArgs = {
  gt?: InputMaybe<Scalars['Int']['input']>;
  gte?: InputMaybe<Scalars['Int']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  lt?: InputMaybe<Scalars['Int']['input']>;
  lte?: InputMaybe<Scalars['Int']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type ExpensesAggregateAggregateUpdatedAtArgs = {
  gt?: InputMaybe<Scalars['DateTime']['input']>;
  gte?: InputMaybe<Scalars['DateTime']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
  lt?: InputMaybe<Scalars['DateTime']['input']>;
  lte?: InputMaybe<Scalars['DateTime']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
};

export type ExpensesAggregateAmountAggregationQuery = {
  __typename?: 'ExpensesAggregateAmountAggregationQuery';
  avg?: Maybe<Scalars['Float']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Float']['output']>;
  min?: Maybe<Scalars['Float']['output']>;
  sum?: Maybe<Scalars['Float']['output']>;
};

export type ExpensesAggregateCategoryIdAggregationQuery = {
  __typename?: 'ExpensesAggregateCategoryIdAggregationQuery';
  avg?: Maybe<Scalars['Int']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Int']['output']>;
  min?: Maybe<Scalars['Int']['output']>;
  sum?: Maybe<Scalars['Int']['output']>;
};

export type ExpensesAggregateCreatedAtAggregationQuery = {
  __typename?: 'ExpensesAggregateCreatedAtAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['DateTime']['output']>;
  min?: Maybe<Scalars['DateTime']['output']>;
};

export type ExpensesAggregateDateAggregationQuery = {
  __typename?: 'ExpensesAggregateDateAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['DateTime']['output']>;
  min?: Maybe<Scalars['DateTime']['output']>;
};

export type ExpensesAggregateIdAggregationQuery = {
  __typename?: 'ExpensesAggregateIdAggregationQuery';
  avg?: Maybe<Scalars['Int']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Int']['output']>;
  min?: Maybe<Scalars['Int']['output']>;
  sum?: Maybe<Scalars['Int']['output']>;
};

export type ExpensesAggregateUpdatedAtAggregationQuery = {
  __typename?: 'ExpensesAggregateUpdatedAtAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['DateTime']['output']>;
  min?: Maybe<Scalars['DateTime']['output']>;
};

export type MetaCreateInput = {
  birth_date?: InputMaybe<Scalars['DateTime']['input']>;
  employee_id?: InputMaybe<Scalars['Int']['input']>;
  general_info?: InputMaybe<Scalars['String']['input']>;
  join_date?: InputMaybe<Scalars['DateTime']['input']>;
  leave_date?: InputMaybe<Scalars['DateTime']['input']>;
  primary_language?: InputMaybe<Scalars['String']['input']>;
  secondary_language?: InputMaybe<Scalars['String']['input']>;
  tin?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
  yt_profile_id?: InputMaybe<Scalars['String']['input']>;
};

export type MetaUpdateInput = {
  birth_date?: InputMaybe<Scalars['DateTime']['input']>;
  employee_id?: InputMaybe<Scalars['Int']['input']>;
  general_info?: InputMaybe<Scalars['String']['input']>;
  join_date?: InputMaybe<Scalars['DateTime']['input']>;
  leave_date?: InputMaybe<Scalars['DateTime']['input']>;
  primary_language?: InputMaybe<Scalars['String']['input']>;
  secondary_language?: InputMaybe<Scalars['String']['input']>;
  tin?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
  yt_profile_id?: InputMaybe<Scalars['String']['input']>;
};

export type Mutation = {
  __typename?: 'Mutation';
  authenticate?: Maybe<AuthPayload>;
  /** Create a record */
  createEmployee?: Maybe<EmployeeType>;
  /** Create a record */
  createExpense?: Maybe<ExpenseType>;
  /** Create a record */
  createExpenseCategory?: Maybe<ExpenseCategoryType>;
  /** Create a record */
  createPosition?: Maybe<PositionType>;
  /** Create a record */
  createTaskType?: Maybe<TaskTypeType>;
  /** Create a new user */
  createUser?: Maybe<UserResponse>;
  /** Delete a record */
  deleteEmployee?: Maybe<Scalars['String']['output']>;
  /** Delete a record */
  deleteExpense?: Maybe<Scalars['String']['output']>;
  /** Delete a record */
  deleteExpenseCategory?: Maybe<Scalars['String']['output']>;
  /** Delete a record */
  deletePosition?: Maybe<Scalars['String']['output']>;
  /** Delete a record */
  deleteTaskType?: Maybe<Scalars['String']['output']>;
  /** Delete a user */
  deleteUser?: Maybe<Scalars['Boolean']['output']>;
  /** Update a record */
  updateEmployee?: Maybe<EmployeeType>;
  /** Update a record */
  updateExpense?: Maybe<ExpenseType>;
  /** Update a record */
  updateExpenseCategory?: Maybe<ExpenseCategoryType>;
  /** Update a record */
  updatePosition?: Maybe<PositionType>;
  /** Update a record */
  updateTaskType?: Maybe<TaskTypeType>;
  /** Update a user */
  updateUser?: Maybe<UserResponse>;
};


export type MutationAuthenticateArgs = {
  email: Scalars['String']['input'];
  password: Scalars['String']['input'];
};


export type MutationCreateEmployeeArgs = {
  data?: InputMaybe<CreateEmployeeInput>;
};


export type MutationCreateExpenseArgs = {
  data?: InputMaybe<CreateExpenseInput>;
};


export type MutationCreateExpenseCategoryArgs = {
  data?: InputMaybe<CreateExpenseCategoryInput>;
};


export type MutationCreatePositionArgs = {
  data?: InputMaybe<CreatePositionInput>;
};


export type MutationCreateTaskTypeArgs = {
  data?: InputMaybe<CreateTaskTypeInput>;
};


export type MutationCreateUserArgs = {
  data: CreateUserInput;
};


export type MutationDeleteEmployeeArgs = {
  Id?: InputMaybe<Scalars['Int']['input']>;
};


export type MutationDeleteExpenseArgs = {
  Id?: InputMaybe<Scalars['Int']['input']>;
};


export type MutationDeleteExpenseCategoryArgs = {
  Id?: InputMaybe<Scalars['Int']['input']>;
};


export type MutationDeletePositionArgs = {
  Id?: InputMaybe<Scalars['Int']['input']>;
};


export type MutationDeleteTaskTypeArgs = {
  Id?: InputMaybe<Scalars['Int']['input']>;
};


export type MutationDeleteUserArgs = {
  id: Scalars['Int']['input'];
};


export type MutationUpdateEmployeeArgs = {
  data?: InputMaybe<UpdateEmployeeInput>;
  id: Scalars['Int']['input'];
};


export type MutationUpdateExpenseArgs = {
  data?: InputMaybe<UpdateExpenseInput>;
  id: Scalars['Int']['input'];
};


export type MutationUpdateExpenseCategoryArgs = {
  data?: InputMaybe<UpdateExpenseCategoryInput>;
  id: Scalars['Int']['input'];
};


export type MutationUpdatePositionArgs = {
  data?: InputMaybe<UpdatePositionInput>;
  id: Scalars['Int']['input'];
};


export type MutationUpdateTaskTypeArgs = {
  data?: InputMaybe<UpdateTaskTypeInput>;
  id: Scalars['Int']['input'];
};


export type MutationUpdateUserArgs = {
  data: UpdateUserInput;
  id: Scalars['Int']['input'];
};

export type Position = {
  __typename?: 'Position';
  created_at?: Maybe<Scalars['DateTime']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type PositionCreateInput = {
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type PositionType = {
  __typename?: 'PositionType';
  created_at?: Maybe<Scalars['DateTime']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type PositionUpdateInput = {
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type PositionsAggregateAggregate = {
  __typename?: 'PositionsAggregateAggregate';
  CreatedAt?: Maybe<PositionsAggregateCreatedAtAggregationQuery>;
  Description?: Maybe<PositionsAggregateDescriptionAggregationQuery>;
  Id?: Maybe<PositionsAggregateIdAggregationQuery>;
  Name?: Maybe<PositionsAggregateNameAggregationQuery>;
  UpdatedAt?: Maybe<PositionsAggregateUpdatedAtAggregationQuery>;
};


export type PositionsAggregateAggregateCreatedAtArgs = {
  gt?: InputMaybe<Scalars['DateTime']['input']>;
  gte?: InputMaybe<Scalars['DateTime']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
  lt?: InputMaybe<Scalars['DateTime']['input']>;
  lte?: InputMaybe<Scalars['DateTime']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
};


export type PositionsAggregateAggregateDescriptionArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type PositionsAggregateAggregateIdArgs = {
  gt?: InputMaybe<Scalars['Int']['input']>;
  gte?: InputMaybe<Scalars['Int']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  lt?: InputMaybe<Scalars['Int']['input']>;
  lte?: InputMaybe<Scalars['Int']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type PositionsAggregateAggregateNameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type PositionsAggregateAggregateUpdatedAtArgs = {
  gt?: InputMaybe<Scalars['DateTime']['input']>;
  gte?: InputMaybe<Scalars['DateTime']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
  lt?: InputMaybe<Scalars['DateTime']['input']>;
  lte?: InputMaybe<Scalars['DateTime']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
};

export type PositionsAggregateCreatedAtAggregationQuery = {
  __typename?: 'PositionsAggregateCreatedAtAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['DateTime']['output']>;
  min?: Maybe<Scalars['DateTime']['output']>;
};

export type PositionsAggregateDescriptionAggregationQuery = {
  __typename?: 'PositionsAggregateDescriptionAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type PositionsAggregateIdAggregationQuery = {
  __typename?: 'PositionsAggregateIdAggregationQuery';
  avg?: Maybe<Scalars['Int']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Int']['output']>;
  min?: Maybe<Scalars['Int']['output']>;
  sum?: Maybe<Scalars['Int']['output']>;
};

export type PositionsAggregateNameAggregationQuery = {
  __typename?: 'PositionsAggregateNameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type PositionsAggregateUpdatedAtAggregationQuery = {
  __typename?: 'PositionsAggregateUpdatedAtAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['DateTime']['output']>;
  min?: Maybe<Scalars['DateTime']['output']>;
};

export type Query = {
  __typename?: 'Query';
  /** Get by id */
  employee?: Maybe<Employee>;
  /** Get paginated */
  employees?: Maybe<Employees>;
  /** Get aggregated data */
  employeesAggregate?: Maybe<Array<Maybe<EmployeesAggregateAggregate>>>;
  /** Get by id */
  expense?: Maybe<Expense>;
  /** Get paginated */
  expenseCategories?: Maybe<ExpenseCategories>;
  /** Get aggregated data */
  expenseCategoriesAggregate?: Maybe<Array<Maybe<ExpenseCategoriesAggregateAggregate>>>;
  /** Get by id */
  expenseCategory?: Maybe<ExpenseCategory>;
  /** Get paginated */
  expenses?: Maybe<Expenses>;
  /** Get aggregated data */
  expensesAggregate?: Maybe<Array<Maybe<ExpensesAggregateAggregate>>>;
  /** Get by id */
  position?: Maybe<Position>;
  /** Get paginated */
  positions?: Maybe<Positions>;
  /** Get aggregated data */
  positionsAggregate?: Maybe<Array<Maybe<PositionsAggregateAggregate>>>;
  /** Get by id */
  taskType?: Maybe<TaskType>;
  /** Get paginated */
  taskTypes?: Maybe<TaskTypes>;
  /** Get aggregated data */
  taskTypesAggregate?: Maybe<Array<Maybe<TaskTypesAggregateAggregate>>>;
  /** Get by id */
  user?: Maybe<User>;
  /** Get paginated */
  users?: Maybe<Users>;
};


export type QueryEmployeeArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryEmployeesArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryEmployeesAggregateArgs = {
  groupBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryExpenseArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryExpenseCategoriesArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryExpenseCategoriesAggregateArgs = {
  groupBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryExpenseCategoryArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryExpensesArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryExpensesAggregateArgs = {
  groupBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryPositionArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryPositionsArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryPositionsAggregateArgs = {
  groupBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryTaskTypeArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryTaskTypesArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryTaskTypesAggregateArgs = {
  groupBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type QueryUserArgs = {
  id?: InputMaybe<Scalars['Int']['input']>;
};


export type QueryUsersArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  offset?: InputMaybe<Scalars['Int']['input']>;
  sortBy?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};

export type TaskType = {
  __typename?: 'TaskType';
  created_at?: Maybe<Scalars['DateTime']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  icon?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type TaskTypeType = {
  __typename?: 'TaskTypeType';
  created_at?: Maybe<Scalars['DateTime']['output']>;
  description?: Maybe<Scalars['String']['output']>;
  icon?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
};

export type TaskTypesAggregateAggregate = {
  __typename?: 'TaskTypesAggregateAggregate';
  CreatedAt?: Maybe<TaskTypesAggregateCreatedAtAggregationQuery>;
  Description?: Maybe<TaskTypesAggregateDescriptionAggregationQuery>;
  Icon?: Maybe<TaskTypesAggregateIconAggregationQuery>;
  Id?: Maybe<TaskTypesAggregateIdAggregationQuery>;
  Name?: Maybe<TaskTypesAggregateNameAggregationQuery>;
  UpdatedAt?: Maybe<TaskTypesAggregateUpdatedAtAggregationQuery>;
};


export type TaskTypesAggregateAggregateCreatedAtArgs = {
  gt?: InputMaybe<Scalars['DateTime']['input']>;
  gte?: InputMaybe<Scalars['DateTime']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
  lt?: InputMaybe<Scalars['DateTime']['input']>;
  lte?: InputMaybe<Scalars['DateTime']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
};


export type TaskTypesAggregateAggregateDescriptionArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type TaskTypesAggregateAggregateIconArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type TaskTypesAggregateAggregateIdArgs = {
  gt?: InputMaybe<Scalars['Int']['input']>;
  gte?: InputMaybe<Scalars['Int']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
  lt?: InputMaybe<Scalars['Int']['input']>;
  lte?: InputMaybe<Scalars['Int']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['Int']['input']>>>;
};


export type TaskTypesAggregateAggregateNameArgs = {
  in?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
  out?: InputMaybe<Array<InputMaybe<Scalars['String']['input']>>>;
};


export type TaskTypesAggregateAggregateUpdatedAtArgs = {
  gt?: InputMaybe<Scalars['DateTime']['input']>;
  gte?: InputMaybe<Scalars['DateTime']['input']>;
  in?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
  lt?: InputMaybe<Scalars['DateTime']['input']>;
  lte?: InputMaybe<Scalars['DateTime']['input']>;
  out?: InputMaybe<Array<InputMaybe<Scalars['DateTime']['input']>>>;
};

export type TaskTypesAggregateCreatedAtAggregationQuery = {
  __typename?: 'TaskTypesAggregateCreatedAtAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['DateTime']['output']>;
  min?: Maybe<Scalars['DateTime']['output']>;
};

export type TaskTypesAggregateDescriptionAggregationQuery = {
  __typename?: 'TaskTypesAggregateDescriptionAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type TaskTypesAggregateIconAggregationQuery = {
  __typename?: 'TaskTypesAggregateIconAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type TaskTypesAggregateIdAggregationQuery = {
  __typename?: 'TaskTypesAggregateIdAggregationQuery';
  avg?: Maybe<Scalars['Int']['output']>;
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['Int']['output']>;
  min?: Maybe<Scalars['Int']['output']>;
  sum?: Maybe<Scalars['Int']['output']>;
};

export type TaskTypesAggregateNameAggregationQuery = {
  __typename?: 'TaskTypesAggregateNameAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
};

export type TaskTypesAggregateUpdatedAtAggregationQuery = {
  __typename?: 'TaskTypesAggregateUpdatedAtAggregationQuery';
  count?: Maybe<Scalars['Int']['output']>;
  max?: Maybe<Scalars['DateTime']['output']>;
  min?: Maybe<Scalars['DateTime']['output']>;
};

export type UpdateEmployeeInput = {
  avatar_id?: InputMaybe<Scalars['Int']['input']>;
  coefficient?: InputMaybe<Scalars['Float']['input']>;
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  email?: InputMaybe<Scalars['String']['input']>;
  first_name?: InputMaybe<Scalars['String']['input']>;
  hourly_rate?: InputMaybe<Scalars['Float']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  last_name?: InputMaybe<Scalars['String']['input']>;
  meta?: InputMaybe<MetaUpdateInput>;
  middle_name?: InputMaybe<Scalars['String']['input']>;
  phone?: InputMaybe<Scalars['String']['input']>;
  position?: InputMaybe<PositionUpdateInput>;
  position_id?: InputMaybe<Scalars['Int']['input']>;
  salary?: InputMaybe<Scalars['Float']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type UpdateExpenseCategoryInput = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type UpdateExpenseInput = {
  amount?: InputMaybe<Scalars['Float']['input']>;
  category?: InputMaybe<CategoryUpdateInput>;
  category_id?: InputMaybe<Scalars['Int']['input']>;
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  date?: InputMaybe<Scalars['DateTime']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type UpdatePositionInput = {
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type UpdateTaskTypeInput = {
  created_at?: InputMaybe<Scalars['DateTime']['input']>;
  description?: InputMaybe<Scalars['String']['input']>;
  icon?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  name?: InputMaybe<Scalars['String']['input']>;
  updated_at?: InputMaybe<Scalars['DateTime']['input']>;
};

export type UpdateUserInput = {
  avatar?: InputMaybe<AvatarUpdateInput>;
  avatarId?: InputMaybe<Scalars['Int']['input']>;
  createdAt?: InputMaybe<Scalars['DateTime']['input']>;
  email?: InputMaybe<Scalars['String']['input']>;
  employeeId?: InputMaybe<Scalars['Int']['input']>;
  firstName?: InputMaybe<Scalars['String']['input']>;
  id?: InputMaybe<Scalars['Int']['input']>;
  lastAction?: InputMaybe<Scalars['DateTime']['input']>;
  lastIp?: InputMaybe<Scalars['String']['input']>;
  lastLogin?: InputMaybe<Scalars['DateTime']['input']>;
  lastName?: InputMaybe<Scalars['String']['input']>;
  middleName?: InputMaybe<Scalars['String']['input']>;
  password?: InputMaybe<Scalars['String']['input']>;
  updatedAt?: InputMaybe<Scalars['DateTime']['input']>;
};

export type User = {
  __typename?: 'User';
  avatar?: Maybe<UseravatarJoin>;
  avatarId?: Maybe<Scalars['Int']['output']>;
  createdAt?: Maybe<Scalars['DateTime']['output']>;
  email?: Maybe<Scalars['String']['output']>;
  employeeId?: Maybe<Scalars['Int']['output']>;
  firstName?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  lastAction?: Maybe<Scalars['DateTime']['output']>;
  lastIp?: Maybe<Scalars['String']['output']>;
  lastLogin?: Maybe<Scalars['DateTime']['output']>;
  lastName?: Maybe<Scalars['String']['output']>;
  middleName?: Maybe<Scalars['String']['output']>;
  updatedAt?: Maybe<Scalars['DateTime']['output']>;
};

export type UserResponse = {
  __typename?: 'UserResponse';
  avatar?: Maybe<UserResponseavatarJoin>;
  avatarId?: Maybe<Scalars['Int']['output']>;
  createdAt?: Maybe<Scalars['DateTime']['output']>;
  email?: Maybe<Scalars['String']['output']>;
  employeeId?: Maybe<Scalars['Int']['output']>;
  firstName?: Maybe<Scalars['String']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  lastAction?: Maybe<Scalars['DateTime']['output']>;
  lastIp?: Maybe<Scalars['String']['output']>;
  lastLogin?: Maybe<Scalars['DateTime']['output']>;
  lastName?: Maybe<Scalars['String']['output']>;
  middleName?: Maybe<Scalars['String']['output']>;
  updatedAt?: Maybe<Scalars['DateTime']['output']>;
};

export type UserResponseavatarJoin = {
  __typename?: 'UserResponseavatarJoin';
  created_at?: Maybe<Scalars['DateTime']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  mimetype?: Maybe<Scalars['String']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  path?: Maybe<Scalars['String']['output']>;
  size?: Maybe<Scalars['Float']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
  uploader_id?: Maybe<Scalars['Int']['output']>;
};

export type UseravatarJoin = {
  __typename?: 'UseravatarJoin';
  created_at?: Maybe<Scalars['DateTime']['output']>;
  id?: Maybe<Scalars['Int']['output']>;
  mimetype?: Maybe<Scalars['String']['output']>;
  name?: Maybe<Scalars['String']['output']>;
  path?: Maybe<Scalars['String']['output']>;
  size?: Maybe<Scalars['Float']['output']>;
  updated_at?: Maybe<Scalars['DateTime']['output']>;
  uploader_id?: Maybe<Scalars['Int']['output']>;
};

export type Employees = {
  __typename?: 'employees';
  data?: Maybe<Array<Maybe<Employee>>>;
  total?: Maybe<Scalars['Int']['output']>;
};

export type ExpenseCategories = {
  __typename?: 'expenseCategories';
  data?: Maybe<Array<Maybe<ExpenseCategory>>>;
  total?: Maybe<Scalars['Int']['output']>;
};

export type Expenses = {
  __typename?: 'expenses';
  data?: Maybe<Array<Maybe<Expense>>>;
  total?: Maybe<Scalars['Int']['output']>;
};

export type Positions = {
  __typename?: 'positions';
  data?: Maybe<Array<Maybe<Position>>>;
  total?: Maybe<Scalars['Int']['output']>;
};

export type TaskTypes = {
  __typename?: 'taskTypes';
  data?: Maybe<Array<Maybe<TaskType>>>;
  total?: Maybe<Scalars['Int']['output']>;
};

export type Users = {
  __typename?: 'users';
  data?: Maybe<Array<Maybe<User>>>;
  total?: Maybe<Scalars['Int']['output']>;
};
