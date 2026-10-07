import { useState, useEffect } from "react";
import { usePagedList } from "../../PagedList";
import type { Row, PageProps } from "../../shared/types";
export function useUserAccess(data: PageProps["data"], revision: number) {
  const userList = usePagedList(data.user.admin ? "/users" : "", revision);
  const grantList = usePagedList(data.user.admin ? "/grants" : "", revision);
  const [editUser, setEditUser] = useState<Row | null>(null);
  const [addUser, setAddUser] = useState(false);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [member, setMember] = useState(true);
  const [admin, setAdmin] = useState(false);
  const [grantUser, setGrantUser] = useState("");
  const [grantAccount, setGrantAccount] = useState("");
  const [role, setRole] = useState("viewer");
  const [userNameError, setUserNameError] = useState("");
  useEffect(() => setUserNameError(""), [addUser, editUser?.id]);
  return {
    userList,
    grantList,
    editUser,
    setEditUser,
    addUser,
    setAddUser,
    username,
    setUsername,
    password,
    setPassword,
    member,
    setMember,
    admin,
    setAdmin,
    grantUser,
    setGrantUser,
    grantAccount,
    setGrantAccount,
    role,
    setRole,
    userNameError,
    setUserNameError,
  };
}
