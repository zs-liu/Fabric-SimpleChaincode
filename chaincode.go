package main

import (
	"fmt"
	"strings"
	"encoding/json"
	"bytes"
	"time"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

type SimpleChaincode struct {
}

type record struct {
	ObjectType     string    `json:"docType"`
	RecordKey      string    `json:"rkey"`
	PatientKey     string    `json:"pkey"`
	PatientSex     string    `json:"psex"`
	PatientAge     int       `json:"page"`
	DateInfo       time.Time `json:"date"`
	PatientDisease string    `json:"info"`
}

type jsontime time.Time

const timeFormat = "2006-01-02 15:04:05"

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}

func (recordTime jsontime) MarshalJSON() ([]byte, error) {
	var stamp = fmt.Sprintf("\"%s\"", time.Time(recordTime).Format(timeFormat))
	return []byte(stamp), nil
}

func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	fmt.Println("Invoke is running " + function)
	if function == "insertRecord" {
		return t.insertPatient(stub, args)
	} else if function == "queryPatientByKey" {
		return t.queryPatientByKey(stub, args)
	} else if function == "queryPatientByAge" {
		return t.queryRecordByAge(stub, args)
	} else if function == "queryRecordByKey" {
		return t.queryRecordByKey(stub, args)
	} else if function == "queryRecordByDisease" {
		return t.queryRecordByDisease(stub, args)
	} else if function == "queryRecordBySex" {
		return t.queryRecordBySex(stub, args)
	} else if function == "queryPatientHistory" {
		return t.queryPatientHistory(stub, args)
	} else if function == "updatePatientState" {
		return t.updatePatientState(stub, args)
	} else if function == "queryPatientByRange" {
		return t.queryPatientByRange(stub, args)
	}
	fmt.Println("Invoke did not find func: " + function) //not supported func
	return shim.Error("Received unknown function invocation")
}

func (t *SimpleChaincode) insertPatient(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var err error
	//  0       1      2     3             4               5
	// "0123","0000","male","35","2006-01-02 15:04:05","Headache"
	if len(args) != 6 {
		return shim.Error("Insert: incorrect number of arguments. Expecting 4")
	}

	// === Insert Running ===
	fmt.Println("Insert- start insert patient")
	for i := 0; i < 6; i++ {
		if len(args[i]) <= 0 {
			return shim.Error("Insert: " + strconv.Itoa(i) + "th argument must be a non-empty string/integer/time")
		}
	}
	recordKey := args[0]
	patientKey := args[1]
	patientSex := strings.ToLower(args[2])
	patientAge, err := strconv.Atoi(args[3])
	if err != nil {
		return shim.Error("Insert: 3rd argument must be a numeric integer")
	}
	dateInfo, err := time.Parse(timeFormat, args[4])
	if err != nil {
		return shim.Error("Insert: 4rd argument must be in time format")
	}
	patientDisease := strings.ToLower(args[5])

	// === Check if patient exists ===
	patientAsBytes, err := stub.GetState(patientKey)
	if err != nil {
		return shim.Error("Insert: failed to get patient. " + err.Error())
	} else if patientAsBytes != nil {
		fmt.Println("Insert: This patient already exists--" + patientKey)
		return shim.Error("Insert: This patient already exists--" + patientKey)
	}

	// === Create record object and marshal to JSON ===
	objectType := "record"
	record := &record{objectType, recordKey, patientKey, patientSex, patientAge, dateInfo, patientDisease}
	recordJSONasBytes, err := json.Marshal(record)
	if err != nil {
		return shim.Error("Insert: " + err.Error())
	}

	// === Save record to state ===
	err = stub.PutState(patientKey, recordJSONasBytes)
	if err != nil {
		return shim.Error("Insert: " + err.Error())
	}

	// ==== Index the patient to enable age-based range queries, e.g. return all 35-year-old patients ====
	// An 'index' is a normal key/value entry in state.
	// The key is a composite key, with the elements that you want to range query on listed first.
	// In our case, the composite key is based on indexName~patientAge~patientKey.
	// This will enable very efficient state range queries based on composite keys matching indexName~color~*
	indexName := "disease~pkey"
	diseaseKeyIndexKey, err := stub.CreateCompositeKey(indexName, []string{record.PatientDisease, record.PatientKey})
	if err != nil {
		return shim.Error(err.Error())
	}
	// Save index entry to state. Only the key name is needed, no need to store a duplicate copy of the marble.
	// Note - passing a 'nil' value will effectively delete the key from state, therefore we pass null character as value
	value := []byte{0x00}
	stub.PutState(diseaseKeyIndexKey, value)

	// === Record saved and indexed. Return success ====
	fmt.Println("- end init patient")
	return shim.Success(nil)
}

func (t *SimpleChaincode) queryPatientByKey(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var patientKey, jsonResp string
	var err error

	if len(args) != 1 {
		return shim.Error("Query: Incorrect number of arguments. Expecting key of the patient to query")
	}

	patientKey = args[0]
	valAsbytes, err := stub.GetState(patientKey)
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + patientKey + "\"}"
		return shim.Error(jsonResp)
	} else if valAsbytes == nil {
		jsonResp = "{\"Error\":\"Patient does not exist: " + patientKey + "\"}"
		return shim.Error(jsonResp)
	}

	return shim.Success(valAsbytes)
}

func (t *SimpleChaincode) queryRecordByKey(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	//   0
	// "0123"
	if len(args) != 1 {
		return shim.Error("Query: incorrect number of arguments. Expecting 1")
	}

	recordKey := args[0]
	queryString := fmt.Sprintf("{\"selector\":{\"doctype\":\"record\",\"rkey\":\"%s\"}}", recordKey)

	var err error
	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(queryResults)
}

func (t *SimpleChaincode) queryRecordByAge(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	//  0
	// "35"
	if len(args) != 1 {
		return shim.Error("Query: incorrect number of arguments. Expecting 1")
	}
	var err error
	patientAge, err := strconv.Atoi(args[0])
	if err != nil {
		return shim.Error("Query: 1th argument must be a numeric integer")
	}

	queryString := fmt.Sprintf("{\"selector\":{\"doctype\":\"record\",\"page\":%s}}", strconv.Itoa(patientAge))

	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(queryResults)
}

func (t *SimpleChaincode) queryRecordByDisease(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	//     0
	// "headache"
	if len(args) != 1 {
		return shim.Error("Query: incorrect number of arguments. Expecting 1")
	}
	var err error
	patientDisease := strings.ToLower(args[0])

	diseasePatientResultsIterator, err := stub.GetStateByPartialCompositeKey("disease~pkey", []string{patientDisease})
	if err != nil {
		return shim.Error(err.Error())
	}
	defer diseasePatientResultsIterator.Close()

	// Iterate through result set and for each patient found
	var buffer bytes.Buffer
	var jsonResp string
	buffer.WriteString("[")
	for diseasePatientResultsIterator.HasNext() {
		// Note that we don't get the value (2nd return variable), we'll just get the patient key from the composite key
		responseRange, err := diseasePatientResultsIterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}

		// Get the disease and patient key from disease~pkey composite key
		objectType, compositeKeyParts, err := stub.SplitCompositeKey(responseRange.Key)
		if err != nil {
			return shim.Error(err.Error())
		}
		returnedDisease := compositeKeyParts[0]
		returnedPatientKey := compositeKeyParts[1]
		fmt.Printf("- found a patient from index:%s disease:%s patinet key:%s\n", objectType, returnedDisease, returnedPatientKey)
		valAsbytes, err := stub.GetState(returnedPatientKey)
		if err != nil {
			jsonResp = "{\"Error\":\"Failed to get state for " + returnedPatientKey + "\"}"
			return shim.Error(jsonResp)
		} else if valAsbytes == nil {
			jsonResp = "{\"Error\":\"Patient does not exist: " + returnedPatientKey + "\"}"
			return shim.Error(jsonResp)
		}
		buffer.Write(valAsbytes)
		buffer.WriteByte('\n')
	}
	fmt.Printf("- found patients info as follows:%s\n", buffer.String())
	return shim.Success(buffer.Bytes())
}

func (t *SimpleChaincode) queryRecordBySex(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	//   0
	// "male"
	if len(args) != 1 {
		return shim.Error("Query: incorrect number of arguments. Expecting 1")
	}
	var err error
	patientSex := strings.ToLower(args[0])

	queryString := fmt.Sprintf("{\"selector\":{\"doctype\":\"record\",\"psex\":\"%s\"}}", patientSex)

	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(queryResults)
}

func (t *SimpleChaincode) queryPatientHistory(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) < 1 {
		return shim.Error("Query: Incorrect number of arguments. Expecting 1")
	}

	patientKey := args[0]

	fmt.Printf("Query- start query patient's history: %s\n", patientKey)

	resultsIterator, err := stub.GetHistoryForKey(patientKey)
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing historic values for the marble
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		buffer.WriteString("{\"TxId\":")
		buffer.WriteString("\"")
		buffer.WriteString(response.TxId)
		buffer.WriteString("\"")

		buffer.WriteString(", \"Value\":")
		// if it was a delete operation on given key, then we need to set the
		//corresponding value null. Else, we will write the response.Value
		//as-is (as the Value itself a JSON marble)
		if response.IsDelete {
			buffer.WriteString("null")
		} else {
			buffer.WriteString(string(response.Value))
		}

		buffer.WriteString(", \"Timestamp\":")
		buffer.WriteString("\"")
		buffer.WriteString(time.Unix(response.Timestamp.Seconds, int64(response.Timestamp.Nanos)).String())
		buffer.WriteString("\"")

		buffer.WriteString(", \"IsDelete\":")
		buffer.WriteString("\"")
		buffer.WriteString(strconv.FormatBool(response.IsDelete))
		buffer.WriteString("\"")

		buffer.WriteString("}")
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")

	fmt.Printf("- query patient's history returning:\n%s\n", buffer.String())

	return shim.Success(buffer.Bytes())
}

func (t *SimpleChaincode) queryPatientByRange(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) < 2 {
		return shim.Error("RangeQuery: Incorrect number of arguments. Expecting 2")
	}
	startKey := args[0]
	endKey := args[1]
	resultsIterator, err := stub.GetStateByRange(startKey, endKey)
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()
	// buffer is a JSON array containing QueryResults
	var buffer bytes.Buffer
	buffer.WriteString("[")
	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		buffer.WriteString("{\"Key\":")
		buffer.WriteString("\"")
		buffer.WriteString(queryResponse.Key)
		buffer.WriteString("\"")
		buffer.WriteString(", \"Record\":")
		// Record is a JSON object, so we write as-is
		buffer.WriteString(string(queryResponse.Value))
		buffer.WriteString("}")
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")
	fmt.Printf("- queryPatientByRange queryResult:\n%s\n", buffer.String())
	return shim.Success(buffer.Bytes())
}

func getQueryResultForQueryString(stub shim.ChaincodeStubInterface, queryString string) ([]byte, error) {
	fmt.Printf("- getQueryResultForQueryString queryString:\n%s\n", queryString)

	resultsIterator, err := stub.GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing QueryRecords
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		buffer.WriteString("{\"Key\":")
		buffer.WriteString("\"")
		buffer.WriteString(queryResponse.Key)
		buffer.WriteString("\"")

		buffer.WriteString(", \"Record\":")
		// Record is a JSON object, so we write as-is
		buffer.WriteString(string(queryResponse.Value))
		buffer.WriteString("}")
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")
	fmt.Printf("- getQueryResultForQueryString queryResult:\n%s\n", buffer.String())
	return buffer.Bytes(), nil
}

func (t *SimpleChaincode) updatePatientState(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	//    0      1      2            3               4
	// '"0124","0000","35","2006-01-02 15:04:05","legbroke"
	if len(args) < 4 {
		return shim.Error("Update: incorrect number of arguments. Expecting 4")
	}
	var err error
	recordKey := args[0]
	patientKey := args[1]
	newAge, err := strconv.Atoi(args[2])
	if err != nil {
		return shim.Error("Update: 3rd argument must be a numeric integer")
	}
	newDate, err := time.Parse(timeFormat, args[3])
	if err != nil {
		return shim.Error("Update: 4rd argument must be in time format")
	}
	newDisease := strings.ToLower(args[4])
	fmt.Println("Update- start update patient state ", patientKey, ":", recordKey, newAge, newDate, newDisease)

	patientAsBytes, err := stub.GetState(patientKey)
	if err != nil {
		return shim.Error("Update: failed to get patient. " + err.Error())
	} else if patientAsBytes == nil {
		fmt.Println("Update: This patient does not exist--" + patientKey)
		return shim.Error("Update: This patient does not exist--" + patientKey)
	}
	patientToTransfer := record{}
	err = json.Unmarshal(patientAsBytes, &patientToTransfer)
	if err != nil {
		return shim.Error(err.Error())
	}

	// Maintain the index
	indexName := "disease~pkey"
	diseaseKeyIndexKey, err := stub.CreateCompositeKey(indexName, []string{patientToTransfer.PatientDisease, patientToTransfer.PatientKey})
	if err != nil {
		return shim.Error(err.Error())
	}

	// Delete index entry to state
	err = stub.DelState(diseaseKeyIndexKey)
	if err != nil {
		return shim.Error("Update: Failed to maintain index :" + err.Error())
	}
	return shim.Success(nil)

	patientToTransfer.PatientAge = newAge
	patientToTransfer.DateInfo = newDate
	patientToTransfer.PatientDisease = newDisease //change info

	// Create new index entry to state
	newDiseaseKeyIndexKey, err := stub.CreateCompositeKey(indexName, []string{patientToTransfer.PatientDisease, patientToTransfer.PatientKey})
	if err != nil {
		return shim.Error(err.Error())
	}
	// Save index entry to state. Only the key name is needed, no need to store a duplicate copy of the marble.
	// Note - passing a 'nil' value will effectively delete the key from state, therefore we pass null character as value
	value := []byte{0x00}
	stub.PutState(newDiseaseKeyIndexKey, value)

	patientJSONasBytes, _ := json.Marshal(patientToTransfer)
	err = stub.PutState(patientKey, patientJSONasBytes) //rewrite the patient
	if err != nil {
		return shim.Error(err.Error())
	}
	fmt.Println("- end update patient state (success)")
	return shim.Success(nil)
}
