package fp

// Allow to transform a function that receives 2 params in a sequence of unary functions
func Curry2[T1, T2, R any](fn func(T1, T2) R) func(T1) func(T2) R {
	return func(t1 T1) func(T2) R {
		return func(t2 T2) R {
			return fn(t1, t2)
		}
	}
}

// Allow to transform a function that receives 3 params in a sequence of unary functions
func Curry3[T1, T2, T3, R any](fn func(T1, T2, T3) R) func(T1) func(T2) func(T3) R {
	return func(t1 T1) func(T2) func(T3) R {
		return func(t2 T2) func(T3) R {
			return func(t3 T3) R {
				return fn(t1, t2, t3)
			}
		}
	}
}

// Allow to transform a function that receives 4 params in a sequence of unary functions
func Curry4[T1, T2, T3, T4, R any](fn func(T1, T2, T3, T4) R) func(T1) func(T2) func(T3) func(T4) R {
	return func(t1 T1) func(T2) func(T3) func(T4) R {
		return func(t2 T2) func(T3) func(T4) R {
			return func(t3 T3) func(T4) R {
				return func(t4 T4) R {
					return fn(t1, t2, t3, t4)
				}
			}
		}
	}
}

// Allow to transform a function that receives 5 params in a sequence of unary functions
func Curry5[T1, T2, T3, T4, T5, R any](fn func(T1, T2, T3, T4, T5) R) func(T1) func(T2) func(T3) func(T4) func(T5) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) R {
		return func(t2 T2) func(T3) func(T4) func(T5) R {
			return func(t3 T3) func(T4) func(T5) R {
				return func(t4 T4) func(T5) R {
					return func(t5 T5) R {
						return fn(t1, t2, t3, t4, t5)
					}
				}
			}
		}
	}
}

// Allow to transform a function that receives 6 params in a sequence of unary functions
func Curry6[T1, T2, T3, T4, T5, T6, R any](fn func(T1, T2, T3, T4, T5, T6) R) func(T1) func(T2) func(T3) func(T4) func(T5) func(T6) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) func(T6) R {
		return func(t2 T2) func(T3) func(T4) func(T5) func(T6) R {
			return func(t3 T3) func(T4) func(T5) func(T6) R {
				return func(t4 T4) func(T5) func(T6) R {
					return func(t5 T5) func(T6) R {
						return func(t6 T6) R {
							return fn(t1, t2, t3, t4, t5, t6)
						}
					}
				}
			}
		}
	}
}

// Allow to transform a function that receives 7 params in a sequence of unary functions
func Curry7[T1, T2, T3, T4, T5, T6, T7, R any](fn func(T1, T2, T3, T4, T5, T6, T7) R) func(T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) R {
		return func(t2 T2) func(T3) func(T4) func(T5) func(T6) func(T7) R {
			return func(t3 T3) func(T4) func(T5) func(T6) func(T7) R {
				return func(t4 T4) func(T5) func(T6) func(T7) R {
					return func(t5 T5) func(T6) func(T7) R {
						return func(t6 T6) func(T7) R {
							return func(t7 T7) R {
								return fn(t1, t2, t3, t4, t5, t6, t7)
							}
						}
					}
				}
			}
		}
	}
}

// Allow to transform a function that receives 8 params in a sequence of unary functions
func Curry8[T1, T2, T3, T4, T5, T6, T7, T8, R any](fn func(T1, T2, T3, T4, T5, T6, T7, T8) R) func(T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) R {
		return func(t2 T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) R {
			return func(t3 T3) func(T4) func(T5) func(T6) func(T7) func(T8) R {
				return func(t4 T4) func(T5) func(T6) func(T7) func(T8) R {
					return func(t5 T5) func(T6) func(T7) func(T8) R {
						return func(t6 T6) func(T7) func(T8) R {
							return func(t7 T7) func(T8) R {
								return func(t8 T8) R {
									return fn(t1, t2, t3, t4, t5, t6, t7, t8)
								}
							}
						}
					}
				}
			}
		}
	}
}

// Allow to transform a function that receives 9 params in a sequence of unary functions
func Curry9[T1, T2, T3, T4, T5, T6, T7, T8, T9, R any](fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9) R) func(T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) R {
		return func(t2 T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) R {
			return func(t3 T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) R {
				return func(t4 T4) func(T5) func(T6) func(T7) func(T8) func(T9) R {
					return func(t5 T5) func(T6) func(T7) func(T8) func(T9) R {
						return func(t6 T6) func(T7) func(T8) func(T9) R {
							return func(t7 T7) func(T8) func(T9) R {
								return func(t8 T8) func(T9) R {
									return func(t9 T9) R {
										return fn(t1, t2, t3, t4, t5, t6, t7, t8, t9)
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

// Allow to transform a function that receives 10 params in a sequence of unary functions
func Curry10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, R any](fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10) R) func(T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) R {
		return func(t2 T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) R {
			return func(t3 T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) R {
				return func(t4 T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) R {
					return func(t5 T5) func(T6) func(T7) func(T8) func(T9) func(T10) R {
						return func(t6 T6) func(T7) func(T8) func(T9) func(T10) R {
							return func(t7 T7) func(T8) func(T9) func(T10) R {
								return func(t8 T8) func(T9) func(T10) R {
									return func(t9 T9) func(T10) R {
										return func(t10 T10) R {
											return fn(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

// Allow to transform a function that receives 11 params in a sequence of unary functions
func Curry11[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, R any](fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11) R) func(T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) R {
		return func(t2 T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) R {
			return func(t3 T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) R {
				return func(t4 T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) R {
					return func(t5 T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) R {
						return func(t6 T6) func(T7) func(T8) func(T9) func(T10) func(T11) R {
							return func(t7 T7) func(T8) func(T9) func(T10) func(T11) R {
								return func(t8 T8) func(T9) func(T10) func(T11) R {
									return func(t9 T9) func(T10) func(T11) R {
										return func(t10 T10) func(T11) R {
											return func(t11 T11) R {
												return fn(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11)
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

// Allow to transform a function that receives 12 params in a sequence of unary functions
func Curry12[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, R any](fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12) R) func(T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) R {
		return func(t2 T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) R {
			return func(t3 T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) R {
				return func(t4 T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) R {
					return func(t5 T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) R {
						return func(t6 T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) R {
							return func(t7 T7) func(T8) func(T9) func(T10) func(T11) func(T12) R {
								return func(t8 T8) func(T9) func(T10) func(T11) func(T12) R {
									return func(t9 T9) func(T10) func(T11) func(T12) R {
										return func(t10 T10) func(T11) func(T12) R {
											return func(t11 T11) func(T12) R {
												return func(t12 T12) R {
													return fn(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12)
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

// Allow to transform a function that receives 13 params in a sequence of unary functions
func Curry13[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, R any](fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13) R) func(T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) R {
		return func(t2 T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) R {
			return func(t3 T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) R {
				return func(t4 T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) R {
					return func(t5 T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) R {
						return func(t6 T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) R {
							return func(t7 T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) R {
								return func(t8 T8) func(T9) func(T10) func(T11) func(T12) func(T13) R {
									return func(t9 T9) func(T10) func(T11) func(T12) func(T13) R {
										return func(t10 T10) func(T11) func(T12) func(T13) R {
											return func(t11 T11) func(T12) func(T13) R {
												return func(t12 T12) func(T13) R {
													return func(t13 T13) R {
														return fn(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12, t13)
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

// Allow to transform a function that receives 14 params in a sequence of unary functions
func Curry14[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, R any](fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14) R) func(T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) R {
		return func(t2 T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) R {
			return func(t3 T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) R {
				return func(t4 T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) R {
					return func(t5 T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) R {
						return func(t6 T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) R {
							return func(t7 T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) R {
								return func(t8 T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) R {
									return func(t9 T9) func(T10) func(T11) func(T12) func(T13) func(T14) R {
										return func(t10 T10) func(T11) func(T12) func(T13) func(T14) R {
											return func(t11 T11) func(T12) func(T13) func(T14) R {
												return func(t12 T12) func(T13) func(T14) R {
													return func(t13 T13) func(T14) R {
														return func(t14 T14) R {
															return fn(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12, t13, t14)
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

// Allow to transform a function that receives 15 params in a sequence of unary functions
func Curry15[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, R any](fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15) R) func(T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) R {
		return func(t2 T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) R {
			return func(t3 T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) R {
				return func(t4 T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) R {
					return func(t5 T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) R {
						return func(t6 T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) R {
							return func(t7 T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) R {
								return func(t8 T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) R {
									return func(t9 T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) R {
										return func(t10 T10) func(T11) func(T12) func(T13) func(T14) func(T15) R {
											return func(t11 T11) func(T12) func(T13) func(T14) func(T15) R {
												return func(t12 T12) func(T13) func(T14) func(T15) R {
													return func(t13 T13) func(T14) func(T15) R {
														return func(t14 T14) func(T15) R {
															return func(t15 T15) R {
																return fn(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12, t13, t14, t15)
															}
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

// Allow to transform a function that receives 16 params in a sequence of unary functions
func Curry16[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16, R any](fn func(T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16) R) func(T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
	return func(t1 T1) func(T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
		return func(t2 T2) func(T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
			return func(t3 T3) func(T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
				return func(t4 T4) func(T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
					return func(t5 T5) func(T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
						return func(t6 T6) func(T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
							return func(t7 T7) func(T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
								return func(t8 T8) func(T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
									return func(t9 T9) func(T10) func(T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
										return func(t10 T10) func(T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
											return func(t11 T11) func(T12) func(T13) func(T14) func(T15) func(T16) R {
												return func(t12 T12) func(T13) func(T14) func(T15) func(T16) R {
													return func(t13 T13) func(T14) func(T15) func(T16) R {
														return func(t14 T14) func(T15) func(T16) R {
															return func(t15 T15) func(T16) R {
																return func(t16 T16) R {
																	return fn(t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12, t13, t14, t15, t16)
																}
															}
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}
